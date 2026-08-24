package main

import (
	"fmt"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/saitamau-maximum/maxicloud/internal/controller"
	"github.com/saitamau-maximum/maxicloud/internal/infra/github"
	"github.com/saitamau-maximum/maxicloud/internal/infra/postgres"
	"github.com/saitamau-maximum/maxicloud/internal/infra/registry"
)

var controllerCmd = &cobra.Command{
	Use:   "controller",
	Short: "Start the Kubernetes controller manager",
	RunE:  runController,
}

func runController(cmd *cobra.Command, args []string) error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	setupLog := ctrl.Log.WithName("setup")

	var cfg ControllerConfig
	if err := env.Parse(&cfg); err != nil {
		return err
	}

	privateKey, err := os.ReadFile(cfg.GitHubPrivateKeyPath)
	if err != nil {
		return err
	}
	ghClient := github.NewClient(cfg.GitHubAppID, privateKey, cfg.InstallationID)

	dsn := postgresDSN(
		cfg.PostgreSQLUser,
		cfg.PostgreSQLPassword,
		cfg.PostgreSQLHost,
		cfg.PostgreSQLPort,
		cfg.PostgreSQLDB,
	)
	pool, err := postgres.NewPool(cmd.Context(), dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	historyRepo := postgres.NewDeploymentHistoryRepository(pool)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: cfg.HealthProbeBindAddress,
	})
	if err != nil {
		return err
	}
	registryClient, err := registry.NewFromConfig(registry.Config{
		Provider: registry.ProviderLocalRegistry,
		Host:     cfg.RegistryHost,
		Password: cfg.RegistryPassword,
	})
	if err != nil {
		return err
	}

	if err := (&controller.ApplicationReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Registry: registryClient,
		Config: controller.ReconcilerConfig{
			IngressClass: cfg.IngressClass,
			BaseDomain:   cfg.BaseDomain,
		},
	}).SetupWithManager(mgr); err != nil {
		return err
	}
	if err := (&controller.BuildRunReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Registry:      registryClient,
		GitHubClient:  ghClient,
		PackVolumeKey: cfg.PackVolumeKey,
	}).SetupWithManager(mgr); err != nil {
		return err
	}
	if err := (&controller.DeployRunReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Reporter:   ghClient,
		DeployRepo: historyRepo,
	}).SetupWithManager(mgr); err != nil {
		return err
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}

	setupLog.Info("Starting controller manager")
	return mgr.Start(ctrl.SetupSignalHandler())
}
