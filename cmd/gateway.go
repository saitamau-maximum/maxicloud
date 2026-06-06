package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
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

	dsn := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s",
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
	logStreamer := k8s.NewLogStreamer(clientset)
	deployRepo := k8s.NewDeployRepository(k8sClient, logStreamer)
	srcRepo := github.NewClient(cfg.GitHubAppID, privateKey, cfg.InstallationID)
	oidcClient := oidc.NewClient(oidc.Config{
		Issuer:       cfg.OIDCIssuer,
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
	})
	allowedRedirects := splitCSV(cfg.AllowedRedirects)

	authSvc := service.NewAuthService(service.AuthConfig{
		SessionSecret:    cfg.SessionSecret,
		AllowedRedirects: allowedRedirects,
	}, userRepo, oidcClient)

	deploySvc := deployment.NewDeploymentService(historyRepo, deployRepo)
	deployEventSvc := deployment.NewDeploymentEventService(appRepo, deploySvc)
	deployHistory := deployment.NewHistory(historyRepo)
	deployWatcher := deployment.NewWatcher(deployHistory, deployRepo)
	userSvc := service.NewUserService(userRepo)
	prjSvc := service.NewProjectService(prjRepo)
	domainSvc := service.NewDomainService(appRepo, strings.Split(cfg.AvailableDomains, ","))
	srcSvc := service.NewSourceService(srcRepo)
	appSvc := service.NewApplicationService(appRepo, deploySvc, srcSvc)

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

	authOpt := connect.WithInterceptors(auth.NewAuthInterceptor(cfg.SessionSecret))
	mountAll(r,
		route(maxicloudv1connect.NewProjectServiceHandler(prjHandler, authOpt)),
		route(maxicloudv1connect.NewUserServiceHandler(userHandler, authOpt)),
		route(maxicloudv1connect.NewApplicationServiceHandler(appHandler, authOpt)),
		route(maxicloudv1connect.NewDeploymentServiceHandler(deployHandler, authOpt)),
		route(maxicloudv1connect.NewGitHubServiceHandler(ghHandler, authOpt)),
		route(maxicloudv1connect.NewDomainServiceHandler(domainHandler, authOpt)),
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

type connectRoute struct {
	path    string
	handler http.Handler
}

func route(path string, h http.Handler) connectRoute {
	return connectRoute{path: path, handler: h}
}

func mountAll(r chi.Router, routes ...connectRoute) {
	for _, route := range routes {
		r.Mount(route.path, route.handler)
	}
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}

func allowedOrigins(redirects []string) []string {
	origins := make([]string, 0, len(redirects))
	seen := map[string]struct{}{}
	for _, redirect := range redirects {
		u, err := url.Parse(redirect)
		if err != nil || u.Scheme == "" || u.Host == "" {
			continue
		}
		origin := u.Scheme + "://" + u.Host
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins
}
