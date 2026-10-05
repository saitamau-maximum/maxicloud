package authz

import (
	"context"

	"github.com/saitamau-maximum/maxicloud/internal/auth"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

// Authorizer は「caller がそのプロジェクトで permission を持つか」を判定する。
type Authorizer interface {
	Authorize(ctx context.Context, projectID string, perm domain.Permission) error
}

type authorizer struct {
	projectRepo   domain.ProjectRepository
	memberRepo    domain.ProjectMemberRepository
	groupRoleRepo domain.ProjectGroupRoleRepository
}

var _ Authorizer = (*authorizer)(nil)

func New(
	projectRepo domain.ProjectRepository,
	memberRepo domain.ProjectMemberRepository,
	groupRoleRepo domain.ProjectGroupRoleRepository,
) Authorizer {
	return &authorizer{
		projectRepo:   projectRepo,
		memberRepo:    memberRepo,
		groupRoleRepo: groupRoleRepo,
	}
}

func (a *authorizer) Authorize(ctx context.Context, projectID string, perm domain.Permission) error {
	project, err := a.projectRepo.Get(ctx, projectID)
	if err != nil {
		return err
	}
	if project == nil {
		return domain.ValidationError{Message: "project not found"}
	}

	p := domain.Principal{
		ID:    auth.UserID(ctx),
		Roles: auth.Roles(ctx),
	}

	var granted []domain.Role

	member, err := a.memberRepo.GetByUser(ctx, project.ID, p.ID)
	if err != nil {
		return err
	}
	if member != nil {
		granted = append(granted, member.Role)
	}

	groupRoles, err := a.groupRoleRepo.ListByGroups(ctx, project.ID, p.Roles)
	if err != nil {
		return err
	}
	for _, g := range groupRoles {
		granted = append(granted, g.Role)
	}

	if !p.Can(perm, project, granted) {
		return domain.ForbiddenError{Message: "permission denied"}
	}
	return nil
}
