package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
	"github.com/saitamau-maximum/maxicloud/internal/handler"
	"github.com/saitamau-maximum/maxicloud/internal/infra/github"
	"github.com/saitamau-maximum/maxicloud/internal/infra/inmemory"
	"github.com/saitamau-maximum/maxicloud/internal/infra/k8s"
	"github.com/saitamau-maximum/maxicloud/internal/usecase"
	deployuc "github.com/saitamau-maximum/maxicloud/internal/usecase/deployment"
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the API gateway server",
	RunE:  runGateway,
}

type connectSvc struct {
	path    string
	handler http.Handler
}

func svc(path string, h http.Handler) connectSvc {
	return connectSvc{path, h}
}

func mountAll(r chi.Router, svcs ...connectSvc) {
	for _, s := range svcs {
		r.Mount(s.path, s.handler)
	}
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

	appRepo := k8s.NewApplicationRepository(k8sClient, cfg.IngressClass)
	prjRepo := k8s.NewProjectRepository(k8sClient)
	historyRepo := inmemory.NewDeploymentHistoryRepository()
	userRepo := inmemory.NewUserRepository()
	logStreamer := k8s.NewLogStreamer(clientset)
	deployRepo := k8s.NewDeployRepository(k8sClient, logStreamer)
	srcRepo := github.NewClient(cfg.GitHubAppID, privateKey, cfg.InstallationID)

	// authSvc := usecase.NewAuthService(usecase.AuthConfig{
	// 	Issuer:        cfg.OIDCIssuer,
	// 	ClientID:      cfg.OIDCClientID,
	// 	RedirectURL:   cfg.OIDCRedirectURL,
	// 	StateSecret:   cfg.StateSecret,
	// 	SessionSecret: cfg.SessionSecret,
	// }, userRepo)

	deploySvc := deployuc.NewDeploymentService(historyRepo, deployRepo)
	deployEventSvc := deployuc.NewDeploymentEventService(appRepo, deploySvc)
	deployHistory := deployuc.NewHistory(historyRepo)
	deployWatcher := deployuc.NewWatcher(deployHistory, deployRepo)
	userSvc := usecase.NewUserService(userRepo)
	prjSvc := usecase.NewProjectUsecase(prjRepo)
	domainSvc := usecase.NewDomainService(appRepo, strings.Split(cfg.AvailableDomains, ","))
	srcSvc := usecase.NewSourceService(srcRepo)
	appSvc := usecase.NewApplicationService(appRepo, deploySvc, srcSvc)

	// authHandler := handler.NewAuthHandler(authSvc)
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
	// TODO: 公開前にちゃんと設定する
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: false,
	}))

	mountAll(r,
		// svc(maxicloudv1connect.NewAuthServiceHandler(authHandler)),
		svc(maxicloudv1connect.NewProjectServiceHandler(prjHandler)),
		svc(maxicloudv1connect.NewUserServiceHandler(userHandler)),
		svc(maxicloudv1connect.NewApplicationServiceHandler(appHandler)),
		svc(maxicloudv1connect.NewDeploymentServiceHandler(deployHandler)),
		svc(maxicloudv1connect.NewGitHubServiceHandler(ghHandler)),
		svc(maxicloudv1connect.NewDomainServiceHandler(domainHandler)),
	)

	// Auth ハンドラ（ブラウザリダイレクト）
	// r.Get("/auth/login", authHandler.Login)
	// r.Get("/auth/callback", authHandler.Callback)

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
