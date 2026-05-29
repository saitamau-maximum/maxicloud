package handler

import (
	"strings"

	gh "github.com/google/go-github/v72/github"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

// toDeploymentEvent は go-github の webhook payload を usecase 層で扱う domain.DeploymentEvent に正規化する。
// 第二戻り値が false の場合、デプロイ対象外のイベント（黙って 200 OK で返す）。
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
			Branch: pr.GetHead().GetRef(),
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
