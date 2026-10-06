package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/service/authz"
)

type ProjectService interface {
	Create(ctx context.Context, name, description, ownerID string) (*domain.Project, error)
	Get(ctx context.Context, id string) (*domain.Project, error)
	List(ctx context.Context) ([]*domain.Project, error)
	Update(ctx context.Context, params UpdateProjectParams) (*domain.Project, error)
	Delete(ctx context.Context, id string) error
}

type projectService struct {
	repo          domain.ProjectRepository
	memberRepo    domain.ProjectMemberRepository
	groupRoleRepo domain.ProjectGroupRoleRepository
	authz         authz.Authorizer
}

func NewProjectService(
	repo domain.ProjectRepository,
	memberRepo domain.ProjectMemberRepository,
	groupRoleRepo domain.ProjectGroupRoleRepository,
	authorizer authz.Authorizer,
) ProjectService {
	return &projectService{
		repo:          repo,
		memberRepo:    memberRepo,
		groupRoleRepo: groupRoleRepo,
		authz:         authorizer,
	}
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
	if err := u.authz.Authorize(ctx, params.ID, domain.PermissionWriteProject); err != nil {
		return nil, err
	}
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
	if err := u.authz.Authorize(ctx, id, domain.PermissionDeleteProject); err != nil {
		if domain.IsNotFoundError(err) {
			return nil
		}
		return err
	}
	if err := u.repo.Delete(ctx, id); err != nil {
		return err
	}
	if err := u.memberRepo.RemoveByProject(ctx, id); err != nil {
		return fmt.Errorf("remove project members: %w", err)
	}
	if err := u.groupRoleRepo.RemoveByProject(ctx, id); err != nil {
		return fmt.Errorf("remove project group roles: %w", err)
	}
	return nil
}
