package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type ProjectService interface {
	Create(ctx context.Context, name, description, ownerID string) (*domain.Project, error)
	Get(ctx context.Context, id string) (*domain.Project, error)
	List(ctx context.Context) ([]*domain.Project, error)
	Update(ctx context.Context, params UpdateProjectParams) (*domain.Project, error)
	Delete(ctx context.Context, id string) error
}

type projectService struct {
	repo domain.ProjectRepository
}

func NewProjectService(repo domain.ProjectRepository) ProjectService {
	return &projectService{repo: repo}
}

func (u *projectService) Create(ctx context.Context, name, description, ownerID string) (*domain.Project, error) {
	id, err := u.repo.Create(ctx, domain.Project{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		OwnerID:     ownerID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})
	if err != nil {
		return nil, err
	}
	return u.repo.Get(ctx, id)
}

func (u *projectService) Get(ctx context.Context, id string) (*domain.Project, error) {
	return u.repo.Get(ctx, id)
}

func (u *projectService) List(ctx context.Context) ([]*domain.Project, error) {
	return u.repo.List(ctx)
}

type UpdateProjectParams struct {
	ID          string
	Name        *string
	Description *string
}

func (u *projectService) Update(ctx context.Context, params UpdateProjectParams) (*domain.Project, error) {
	if err := u.repo.Update(ctx, domain.UpdateProjectParams{
		ID:          params.ID,
		Name:        params.Name,
		Description: params.Description,
		UpdatedAt:   time.Now(),
	}); err != nil {
		return nil, err
	}
	return u.repo.Get(ctx, params.ID)
}

func (u *projectService) Delete(ctx context.Context, id string) error {
	return u.repo.Delete(ctx, id)
}
