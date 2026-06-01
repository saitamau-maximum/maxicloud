package github

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	gh "github.com/google/go-github/v72/github"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

func (c *client) ListRepositories(ctx context.Context) ([]domain.Repository, error) {
	ghClient, err := c.newGHClient()
	if err != nil {
		return nil, err
	}
	// TODO: ページング対応
	opt := &gh.ListOptions{
		Page:    1,
		PerPage: 100,
	}
	result := make([]domain.Repository, 0, 100)

	for {
		res, resp, err := ghClient.Apps.ListRepos(ctx, opt)
		if err != nil {
			return nil, err
		}

		for _, r := range res.Repositories {
			result = append(result, domain.Repository{
				Owner: r.GetOwner().GetLogin(),
				Name:  r.GetName(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return result, nil
}

func (c *client) ListBranches(ctx context.Context, repo domain.Repository) ([]string, error) {
	ghClient, err := c.newGHClient()
	if err != nil {
		return nil, err
	}
	branches, _, err := ghClient.Repositories.ListBranches(ctx, repo.Owner, repo.Name, nil)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(branches))
	for i, b := range branches {
		result[i] = b.GetName()
	}
	return result, nil
}

func (c *client) GetHeadCommit(ctx context.Context, repo domain.Repository, branch string) (domain.Commit, error) {
	ghClient, err := c.newGHClient()
	if err != nil {
		return domain.Commit{}, err
	}
	commit, _, err := ghClient.Repositories.GetCommit(ctx, repo.Owner, repo.Name, branch, nil)
	if err != nil {
		return domain.Commit{}, err
	}

	authorName := ""
	timestamp := time.Now()
	if commit.GetCommit() != nil && commit.GetCommit().GetAuthor() != nil {
		authorName = commit.GetCommit().GetAuthor().GetName()
		if date := commit.GetCommit().GetAuthor().GetDate(); !date.IsZero() {
			timestamp = *date.GetTime()
		}
	}
	if authorName == "" && commit.GetAuthor() != nil {
		authorName = commit.GetAuthor().GetLogin()
	}

	return domain.Commit{
		SHA:        commit.GetSHA(),
		Message:    commit.GetCommit().GetMessage(),
		AuthorName: authorName,
		Timestamp:  timestamp,
	}, nil
}

func ParseRepoURL(repoURL string) (owner, repo string, err error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", err
	}
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", errors.New("invalid GitHub repository URL")
	}
	return parts[0], parts[1], nil
}

func CompareRepositoryURL(repoURL1, repoURL2 string) bool {
	owner1, repo1, err1 := ParseRepoURL(repoURL1)
	owner2, repo2, err2 := ParseRepoURL(repoURL2)
	if err1 != nil || err2 != nil {
		return false
	}
	return owner1 == owner2 && repo1 == repo2
}

func ShortSHA(sha string) string {
	if len(sha) < 8 {
		return sha
	}
	return sha[:8]
}
