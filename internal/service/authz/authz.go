package authz

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/saitamau-maximum/maxicloud/internal/auth"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

// Authorizer は「caller がそのプロジェクトで permission を持つか」を判定する。
// project の存在判定は呼び出し側（Service）の責務とし、ここでは判定のみを純粋に行う。
type Authorizer interface {
	Authorize(ctx context.Context, project *domain.Project, perm domain.Permission) error
}

type authorizer struct {
	memberRepo    domain.ProjectMemberRepository
	groupRoleRepo domain.ProjectGroupRoleRepository
}

var _ Authorizer = (*authorizer)(nil)

func New(memberRepo domain.ProjectMemberRepository, groupRoleRepo domain.ProjectGroupRoleRepository) Authorizer {
	return &authorizer{
		memberRepo:    memberRepo,
		groupRoleRepo: groupRoleRepo,
	}
}

func (a *authorizer) Authorize(ctx context.Context, project *domain.Project, perm domain.Permission) error {
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
