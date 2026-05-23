package handler

import (
	"testing"

	gh "github.com/google/go-github/v72/github"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

func TestToDeploymentEvent_PullRequestUsesBaseBranch(t *testing.T) {
	branch := "main"
	prNumber := 42
	event, ok := toDeploymentEvent(&gh.PullRequestEvent{
		Action: gh.Ptr("opened"),
		Repo: &gh.Repository{
			Owner: &gh.User{Login: gh.Ptr("octocat")},
			Name:  gh.Ptr("hello-world"),
		},
		PullRequest: &gh.PullRequest{
			Number: gh.Ptr(prNumber),
			Title:  gh.Ptr("Add preview"),
			User:   &gh.User{Login: gh.Ptr("alice")},
			Head: &gh.PullRequestBranch{
				Ref: gh.Ptr("feature/preview"),
				SHA: gh.Ptr("head-sha"),
			},
			Base: &gh.PullRequestBranch{
				Ref: gh.Ptr(branch),
			},
		},
	})
	if !ok {
		t.Fatalf("expected pull request event to be handled")
	}
	if event.Type != domain.DeploymentEventTypePreviewRequested {
		t.Fatalf("unexpected event type: %s", event.Type)
	}
	if event.Branch != branch {
		t.Fatalf("expected branch %q, got %q", branch, event.Branch)
	}
	if event.PRNumber == nil || *event.PRNumber != prNumber {
		t.Fatalf("unexpected PR number: %#v", event.PRNumber)
	}
	if event.Commit.SHA != "head-sha" {
		t.Fatalf("unexpected commit sha: %q", event.Commit.SHA)
	}
}
