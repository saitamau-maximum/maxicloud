package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"connectrpc.com/connect"
	gh "github.com/google/go-github/v72/github"
	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1/maxicloudv1connect"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/service"
	"github.com/saitamau-maximum/maxicloud/internal/service/deployment"
	"golang.org/x/oauth2"
)

type GitHubHandlerConfig struct {
	GitHubAppName  string
	WebhookSecret  string
	ClientSecret   string
	ClientID       string
	InstallationID int64
}

type GitHubHandler struct {
	maxicloudv1connect.UnimplementedGitHubServiceHandler
	deployService deployment.DeploymentEventService
	srcService    service.SourceService
	config        GitHubHandlerConfig
	oauthCfg      *oauth2.Config
}

func NewGitHubHandler(deploySvc deployment.DeploymentEventService, srcSvc service.SourceService, config GitHubHandlerConfig) *GitHubHandler {
	oauthCfg := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}
	return &GitHubHandler{
		deployService: deploySvc,
		srcService:    srcSvc,
		config:        config,
		oauthCfg:      oauthCfg,
	}
}

func (h *GitHubHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if code := r.URL.Query().Get("code"); code != "" {
		if _, err := h.oauthCfg.Exchange(r.Context(), code); err != nil {
			http.Error(w, "failed to exchange code for token", http.StatusInternalServerError)
			return
		}
	}

	installationIDStr := r.URL.Query().Get("installation_id")
	if installationIDStr == "" {
		http.Error(w, "installation_id query parameter is required", http.StatusBadRequest)
		return
	}
	installationID, err := strconv.ParseInt(installationIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid installation_id", http.StatusBadRequest)
		return
	}
	if installationID != h.config.InstallationID {
		http.Error(w, "installation_id does not match configured installation", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *GitHubHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	payload, err := gh.ValidatePayload(r, []byte(h.config.WebhookSecret))
	if err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	event, err := gh.ParseWebHook(gh.WebHookType(r), payload)
	if err != nil {
		http.Error(w, "failed to parse webhook payload", http.StatusBadRequest)
		return
	}

	deployEvent, handled := toDeploymentEvent(event)
	// どうでもいいイベントは無視して200 OKを返す
	if !handled {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err := h.deployService.HandleDeploymentEvent(r.Context(), *deployEvent); err != nil {
		if domain.IsValidationError(err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to handle deployment event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *GitHubHandler) ListRepositories(ctx context.Context, req *v1.ListRepositoriesRequest) (*v1.ListRepositoriesResponse, error) {
	repos, err := h.srcService.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}
	var responseRepos []*v1.Repository
	for _, r := range repos {
		responseRepos = append(responseRepos, &v1.Repository{
			Owner: r.Owner,
			Name:  r.Name,
		})
	}
	return &v1.ListRepositoriesResponse{Repositories: responseRepos}, nil
}

func (h *GitHubHandler) ListBranches(ctx context.Context, req *v1.ListBranchesRequest) (*v1.ListBranchesResponse, error) {
	repo := req.GetRepository()
	if repo == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("repository is required"))
	}
	if repo.GetOwner() == "" || repo.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("repository owner and name are required"))
	}

	branches, err := h.srcService.ListBranches(ctx, domain.Repository{
		Owner: repo.GetOwner(),
		Name:  repo.GetName(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.ListBranchesResponse{Branches: branches}, nil
}
