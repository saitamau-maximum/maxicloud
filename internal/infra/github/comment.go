package github

import (
	"context"

	gh "github.com/google/go-github/v72/github"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

func (c *client) GetDeploymentSummary(ctx context.Context, owner, repo string, commentID int64) (*domain.DeploymentSummary, error) {
	ghClient, err := c.newGHClient()
	if err != nil {
		return nil, err
	}

	comment, _, err := ghClient.Issues.GetComment(ctx, owner, repo, commentID)
	if err != nil {
		return nil, err
	}
	return &domain.DeploymentSummary{
		ID:   comment.GetID(),
		Body: comment.GetBody(),
	}, nil
}

func (c *client) CreateDeploymentSummary(ctx context.Context, params domain.CreateDeploymentSummaryParams) (int64, error) {
	ghClient, err := c.newGHClient()
	if err != nil {
		return 0, err
	}

	result, _, err := ghClient.Issues.CreateComment(ctx, params.Owner, params.Repo, params.PrNumber, &gh.IssueComment{
		Body: gh.Ptr(params.Comment),
	})
	if err != nil {
		return 0, err
	}
	return result.GetID(), nil
}

func (c *client) UpdateDeploymentSummary(ctx context.Context, params domain.UpdateDeploymentSummaryParams) error {
	ghClient, err := c.newGHClient()
	if err != nil {
		return err
	}

	_, _, err = ghClient.Issues.EditComment(ctx, params.Owner, params.Repo, params.CommentID, &gh.IssueComment{
		Body: gh.Ptr(params.Comment),
	})
	return err
}
