package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/caarlos0/env/v11"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1/maxicloudv1connect"
	"github.com/saitamau-maximum/maxicloud/internal/auth"
	"github.com/saitamau-maximum/maxicloud/internal/handler"
	"github.com/saitamau-maximum/maxicloud/internal/infra/github"
	"github.com/saitamau-maximum/maxicloud/internal/infra/k8s"
	"github.com/saitamau-maximum/maxicloud/internal/infra/oidc"
	"github.com/saitamau-maximum/maxicloud/internal/infra/postgres"
	"github.com/saitamau-maximum/maxicloud/internal/service"
	"github.com/saitamau-maximum/maxicloud/internal/service/authz"
	"github.com/saitamau-maximum/maxicloud/internal/service/deployment"
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the API gateway server",
	RunE:  runGateway,
}

func runGateway(cmd *cobra.Command, args []string) error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := ctrl.Log.WithName("gateway")

	var cfg GatewayConfig
	if err := env.Parse(&cfg); err != nil {
		return err
	}

	privateKey, err := os.ReadFile(cfg.GitHubPrivateKeyPath)
	if err != nil {
		return err
	}

	restConfig := ctrl.GetConfigOrDie()
	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

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

	appRepo := k8s.NewApplicationRepository(k8sClient, cfg.IngressClass)
	prjRepo := k8s.NewProjectRepository(k8sClient)
	historyRepo := postgres.NewDeploymentHistoryRepository(pool)
	userRepo := postgres.NewUserRepository(pool)
	memberRepo := postgres.NewProjectMemberRepository(pool)
	groupRoleRepo := postgres.NewProjectGroupRoleRepository(pool)
	logStreamer := k8s.NewLogStreamer(clientset)
	deployRepo := k8s.NewDeployRunRepository(k8sClient, logStreamer)
	srcRepo := github.NewClient(cfg.GitHubAppID, privateKey, cfg.InstallationID)
	oidcClient := oidc.NewClient(oidc.Config{
		Issuer:       cfg.OIDCIssuer,
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
	})
	allowedRedirects := splitCSV(cfg.AllowedRedirects)
	if len(allowedRedirects) == 0 {
		return fmt.Errorf("at least one allowed redirect must be specified")
	}

	authSvc := service.NewAuthService(service.AuthConfig{
		SessionSecret:    cfg.SessionSecret,
		AllowedRedirects: allowedRedirects,
	}, userRepo, oidcClient)

	deploySvc := deployment.NewDeploymentService(historyRepo, deployRepo)
	deployEventSvc := deployment.NewDeploymentEventService(appRepo, prjRepo, deploySvc)
	deployHistory := deployment.NewHistory(historyRepo)
	deployWatcher := deployment.NewWatcher(deployHistory, deployRepo)
	authzSvc := authz.New(memberRepo, groupRoleRepo)
	userSvc := service.NewUserService(userRepo)
	prjSvc := service.NewProjectService(prjRepo, authzSvc)
	domainSvc := service.NewDomainService(appRepo, strings.Split(cfg.AvailableDomains, ","))
	srcSvc := service.NewSourceService(srcRepo)
	appSvc := service.NewApplicationService(appRepo, prjRepo, deploySvc, srcSvc, authzSvc)

	authHandler := handler.NewAuthHandler(authSvc)
	ghHandler := handler.NewGitHubHandler(deployEventSvc, srcSvc, handler.GitHubHandlerConfig{
		GitHubAppName:  cfg.GitHubAppName,
		WebhookSecret:  cfg.GitHubWebhookSecret,
		ClientID:       cfg.GitHubClientID,
		ClientSecret:   cfg.GitHubClientSecret,
		InstallationID: cfg.InstallationID,
	})
	prjHandler := handler.NewProjectHandler(prjSvc)
	userHandler := handler.NewUserHandler(userSvc)
	appHandler := handler.NewApplicationHandler(appSvc)
	deployHandler := handler.NewDeploymentHandler(deploySvc, deployHistory, deployWatcher)
	domainHandler := handler.NewDomainHandler(domainSvc)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins(allowedRedirects),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
	}))

	connectOpt := connect.WithInterceptors(
		auth.NewInterceptor(cfg.SessionSecret),
		validate.NewInterceptor(),
	)
	mountAll(r,
		route(maxicloudv1connect.NewProjectServiceHandler(prjHandler, connectOpt)),
		route(maxicloudv1connect.NewUserServiceHandler(userHandler, connectOpt)),
		route(maxicloudv1connect.NewApplicationServiceHandler(appHandler, connectOpt)),
		route(maxicloudv1connect.NewDeploymentServiceHandler(deployHandler, connectOpt)),
		route(maxicloudv1connect.NewGitHubServiceHandler(ghHandler, connectOpt)),
		route(maxicloudv1connect.NewDomainServiceHandler(domainHandler, connectOpt)),
	)

	r.Get("/auth/login", authHandler.Login)
	r.Get("/auth/callback", authHandler.Callback)

	// GitHub App 関連のエンドポイント
	r.Post("/github/webhook", ghHandler.Webhook)
	r.Get("/github/callback", ghHandler.Callback)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("Starting gateway server", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err, "Failed to start gateway server")
		}
	}()

	<-ctx.Done()
	log.Info("Shutting down gateway server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
