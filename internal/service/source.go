package service

import (
	"context"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type SourceService interface {
	ListRepositories(ctx context.Context) ([]domain.Repository, error)
	ListBranches(ctx context.Context, repo domain.Repository) ([]string, error)
	GetHeadCommit(ctx context.Context, repo domain.Repository, branch string) (domain.Commit, error)
}

type sourceService struct {
	srcRepo domain.SourceRepository
}

func NewSourceService(srcRepo domain.SourceRepository) SourceService {
	return &sourceService{
		srcRepo: srcRepo,
	}
}

func (s *sourceService) ListRepositories(ctx context.Context) ([]domain.Repository, error) {
	return s.srcRepo.ListRepositories(ctx)
}

func (s *sourceService) ListBranches(ctx context.Context, repo domain.Repository) ([]string, error) {
	return s.srcRepo.ListBranches(ctx, repo)
}

func (s *sourceService) GetHeadCommit(ctx context.Context, repo domain.Repository, branch string) (domain.Commit, error) {
	return s.srcRepo.GetHeadCommit(ctx, repo, branch)
}
