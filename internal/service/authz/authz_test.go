package authz

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/saitamau-maximum/maxicloud/internal/auth"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type stubProjectRepo struct {
	domain.ProjectRepository
	project *domain.Project
	err     error
}

func (s *stubProjectRepo) Get(_ context.Context, id string) (*domain.Project, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.project == nil || s.project.ID != id {
		return nil, nil
	}
	return s.project, nil
}

type stubMemberRepo struct {
	domain.ProjectMemberRepository
	members []domain.ProjectMember
	err     error
}

func (s *stubMemberRepo) GetByUser(_ context.Context, projectID, userID string) (*domain.ProjectMember, error) {
	if s.err != nil {
		return nil, s.err
	}
	for _, m := range s.members {
		if m.ProjectID == projectID && m.UserID == userID {
			return &m, nil
		}
	}
	return nil, nil
}

type stubGroupRoleRepo struct {
	domain.ProjectGroupRoleRepository
	groupRoles []domain.ProjectGroupRole
	err        error
}

func (s *stubGroupRoleRepo) ListByGroups(_ context.Context, projectID string, oidcRoles []string) ([]domain.ProjectGroupRole, error) {
	if s.err != nil {
		return nil, s.err
	}
	var result []domain.ProjectGroupRole
	for _, g := range s.groupRoles {
		if g.ProjectID == projectID && slices.Contains(oidcRoles, g.OIDCRole) {
			result = append(result, g)
		}
	}
	return result, nil
}

const (
	projectID = "project-1"
	ownerID   = "owner"
	userID    = "user"
)

func TestAuthorize(t *testing.T) {
	t.Parallel()

	project := &domain.Project{ID: projectID, OwnerID: ownerID}
	errDB := errors.New("db error")

	tests := []struct {
		name       string
		userID     string
		roles      []string
		projectID  string
		perm       domain.Permission
		project    *domain.Project
		projectErr error
		members    []domain.ProjectMember
		memberErr  error
		groupRoles []domain.ProjectGroupRole
		groupErr   error
		wantErr    func(error) bool
	}{
		{
			name:      "global admin can delete any project",
			userID:    userID,
			roles:     []string{domain.GlobalAdminRole},
			projectID: projectID,
			perm:      domain.PermissionDeleteProject,
			project:   project,
		},
		{
			name:      "owner can delete own project",
			userID:    ownerID,
			projectID: projectID,
			perm:      domain.PermissionDeleteProject,
			project:   project,
		},
		{
			name:      "user without grants is forbidden",
			userID:    userID,
			projectID: projectID,
			perm:      domain.PermissionWriteApplication,
			project:   project,
			wantErr:   domain.IsForbiddenError,
		},
		{
			name:      "editor member can write application",
			userID:    userID,
			projectID: projectID,
			perm:      domain.PermissionWriteApplication,
			project:   project,
			members:   []domain.ProjectMember{{ProjectID: projectID, UserID: userID, Role: domain.RoleEditor}},
		},
		{
			name:      "editor member cannot delete application",
			userID:    userID,
			projectID: projectID,
			perm:      domain.PermissionDeleteApplication,
			project:   project,
			members:   []domain.ProjectMember{{ProjectID: projectID, UserID: userID, Role: domain.RoleEditor}},
			wantErr:   domain.IsForbiddenError,
		},
		{
			name:      "admin member cannot delete project",
			userID:    userID,
			projectID: projectID,
			perm:      domain.PermissionDeleteProject,
			project:   project,
			members:   []domain.ProjectMember{{ProjectID: projectID, UserID: userID, Role: domain.RoleAdmin}},
			wantErr:   domain.IsForbiddenError,
		},
		{
			name:       "admin group role can delete application",
			userID:     userID,
			roles:      []string{"web"},
			projectID:  projectID,
			perm:       domain.PermissionDeleteApplication,
			project:    project,
			groupRoles: []domain.ProjectGroupRole{{ProjectID: projectID, OIDCRole: "web", Role: domain.RoleAdmin}},
		},
		{
			name:       "group role of another group is not applied",
			userID:     userID,
			roles:      []string{"infra"},
			projectID:  projectID,
			perm:       domain.PermissionWriteApplication,
			project:    project,
			groupRoles: []domain.ProjectGroupRole{{ProjectID: projectID, OIDCRole: "web", Role: domain.RoleAdmin}},
			wantErr:    domain.IsForbiddenError,
		},
		{
			name:      "grants are combined across member and group roles",
			userID:    userID,
			roles:     []string{"web"},
			projectID: projectID,
			perm:      domain.PermissionManageMembers,
			project:   project,
			members:   []domain.ProjectMember{{ProjectID: projectID, UserID: userID, Role: domain.RoleEditor}},
			groupRoles: []domain.ProjectGroupRole{
				{ProjectID: projectID, OIDCRole: "web", Role: domain.RoleAdmin},
			},
		},
		{
			name:      "missing project returns not found",
			userID:    userID,
			roles:     []string{domain.GlobalAdminRole},
			projectID: "missing",
			perm:      domain.PermissionWriteProject,
			project:   project,
			wantErr:   domain.IsNotFoundError,
		},
		{
			name:       "project repository error is returned",
			userID:     ownerID,
			projectID:  projectID,
			perm:       domain.PermissionWriteProject,
			projectErr: errDB,
			wantErr:    func(err error) bool { return errors.Is(err, errDB) },
		},
		{
			name:      "member repository error is returned",
			userID:    userID,
			projectID: projectID,
			perm:      domain.PermissionWriteProject,
			project:   project,
			memberErr: errDB,
			wantErr:   func(err error) bool { return errors.Is(err, errDB) },
		},
		{
			name:      "group role repository error is returned",
			userID:    userID,
			projectID: projectID,
			perm:      domain.PermissionWriteProject,
			project:   project,
			groupErr:  errDB,
			wantErr:   func(err error) bool { return errors.Is(err, errDB) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := New(
				&stubProjectRepo{project: tt.project, err: tt.projectErr},
				&stubMemberRepo{members: tt.members, err: tt.memberErr},
				&stubGroupRoleRepo{groupRoles: tt.groupRoles, err: tt.groupErr},
			)
			ctx := auth.WithRoles(auth.WithUserID(context.Background(), tt.userID), tt.roles)

			err := a.Authorize(ctx, tt.projectID, tt.perm)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Authorize() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !tt.wantErr(err) {
				t.Fatalf("Authorize() error = %v, want matching error", err)
			}
		})
	}
}
