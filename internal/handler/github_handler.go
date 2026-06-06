package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	gh "github.com/google/go-github/v72/github"
	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1/maxicloudv1connect"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/usecase"
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
	deployService usecase.DeploymentService
	srcService    usecase.SourceService
	config        GitHubHandlerConfig
	oauthCfg      *oauth2.Config
}

func NewGitHubHandler(deploySvc usecase.DeploymentService, srcSvc usecase.SourceService, config GitHubHandlerConfig) *GitHubHandler {
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
		log.Printf("webhook: invalid signature: %v", err)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	event, err := gh.ParseWebHook(gh.WebHookType(r), payload)
	if err != nil {
		log.Printf("webhook: failed to parse payload: %v", err)
		http.Error(w, "failed to parse webhook payload", http.StatusBadRequest)
		return
	}

	deployEvent, handled := toDeploymentEvent(event)
	// どうでもいいイベントは無視して200 OKを返す
	if !handled {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err := h.deployService.HandleGitHubEvent(r.Context(), *deployEvent); err != nil {
		if domain.IsValidationError(err) {
			log.Printf("webhook: validation error handling event: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("webhook: failed to handle deployment event: %v", err)
		http.Error(w, "failed to handle deployment event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func toDeploymentEvent(event any) (*domain.DeploymentEvent, bool) {
	switch e := event.(type) {
	case *gh.PushEvent:
		if e.GetDeleted() {
			return nil, false
		}
		const refPrefix = "refs/heads/"
		if !strings.HasPrefix(e.GetRef(), refPrefix) {
			return nil, false
		}
		branch := strings.TrimPrefix(e.GetRef(), refPrefix)
		return &domain.DeploymentEvent{
			Type: domain.DeploymentEventTypeProductionRequested,
			Repo: domain.Repository{
				Owner: e.GetRepo().GetOwner().GetLogin(),
				Name:  e.GetRepo().GetName(),
			},
			Branch: branch,
			Commit: domain.Commit{
				SHA:        e.GetAfter(),
				Message:    e.GetHeadCommit().GetMessage(),
				AuthorName: e.GetHeadCommit().GetAuthor().GetName(),
			},
		}, true
	case *gh.PullRequestEvent:
		var eventType domain.DeploymentEventType
		switch e.GetAction() {
		case "opened", "synchronize", "reopened":
			eventType = domain.DeploymentEventTypePreviewRequested
		case "closed":
			eventType = domain.DeploymentEventTypePreviewDeleted
		default:
			return nil, false
		}
		pr := e.GetPullRequest()
		prNumber := pr.GetNumber()
		return &domain.DeploymentEvent{
			Type: eventType,
			Repo: domain.Repository{
				Owner: e.GetRepo().GetOwner().GetLogin(),
				Name:  e.GetRepo().GetName(),
			},
			Branch: pr.GetBase().GetRef(),
			Commit: domain.Commit{
				SHA:        pr.GetHead().GetSHA(),
				Message:    pr.GetTitle(),
				AuthorName: pr.GetUser().GetLogin(),
			},
			PRNumber: &prNumber,
		}, true
	default:
		return nil, false
	}
}

func (h *GitHubHandler) ListRepositories(ctx context.Context, req *v1.ListRepositoriesRequest) (*v1.ListRepositoriesResponse, error) {
	repos, err := h.srcService.GetRepositories(ctx)
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

	branches, err := h.srcService.GetBranches(ctx, domain.Repository{
		Owner: repo.GetOwner(),
		Name:  repo.GetName(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.ListBranchesResponse{Branches: branches}, nil
}
