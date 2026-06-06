package domain

import (
	"context"
	"time"
)

type Repository struct {
	Owner string // e.g. "saitamau-maximum"
	Name  string // e.g. "maxicloud"
}

func (r Repository) FullName() string {
	return r.Owner + "/" + r.Name
}

type Commit struct {
	SHA        string
	Message    string
	AuthorName string
	Timestamp  time.Time
}

type SourceRepository interface {
	ListRepositories(ctx context.Context) ([]Repository, error)
	ListBranches(ctx context.Context, repo Repository) ([]string, error)
	GetHeadCommit(ctx context.Context, repo Repository, branch string) (Commit, error)
}
