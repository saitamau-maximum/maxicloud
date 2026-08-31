package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/infra/postgres/db"
)

type projectGroupRoleRepository struct {
	q *db.Queries
}

var _ domain.ProjectGroupRoleRepository = (*projectGroupRoleRepository)(nil)

func NewProjectGroupRoleRepository(pool *pgxpool.Pool) domain.ProjectGroupRoleRepository {
	return &projectGroupRoleRepository{q: db.New(pool)}
}

func (r *projectGroupRoleRepository) ListByGroups(ctx context.Context, projectID string, oidcRoles []string) ([]domain.ProjectGroupRole, error) {
	rows, err := r.q.ListProjectGroupRolesByGroups(ctx, db.ListProjectGroupRolesByGroupsParams{
		ProjectID: projectID,
		OidcRoles: oidcRoles,
	})
	if err != nil {
		return nil, fmt.Errorf("list project group roles by groups: %w", err)
	}
	return groupRoleRowsToDomain(rows)
}

func (r *projectGroupRoleRepository) ListByProject(ctx context.Context, projectID string) ([]domain.ProjectGroupRole, error) {
	rows, err := r.q.ListProjectGroupRoles(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project group roles: %w", err)
	}
	return groupRoleRowsToDomain(rows)
}

func (r *projectGroupRoleRepository) Add(ctx context.Context, g domain.ProjectGroupRole) error {
	_, err := r.q.AddProjectGroupRole(ctx, db.AddProjectGroupRoleParams{
		ID:        g.ID,
		ProjectID: g.ProjectID,
		OidcRole:  g.OIDCRole,
		Role:      g.Role.String(),
		CreatedBy: g.CreatedBy,
	})
	return err
}

func (r *projectGroupRoleRepository) Remove(ctx context.Context, id string) error {
	return r.q.RemoveProjectGroupRole(ctx, id)
}

func (r *projectGroupRoleRepository) UpdateRole(ctx context.Context, id string, role domain.Role) error {
	return r.q.UpdateProjectGroupRoleRole(ctx, db.UpdateProjectGroupRoleRoleParams{
		ID:   id,
		Role: role.String(),
	})
}

func groupRoleRowsToDomain(rows []db.ProjectGroupRole) ([]domain.ProjectGroupRole, error) {
	result := make([]domain.ProjectGroupRole, 0, len(rows))
	for _, row := range rows {
		role, err := domain.ParseRole(row.Role)
		if err != nil {
			return nil, fmt.Errorf("parse role: %w", err)
		}
		result = append(result, domain.ProjectGroupRole{
			ID:        row.ID,
			ProjectID: row.ProjectID,
			OIDCRole:  row.OidcRole,
			Role:      role,
			CreatedAt: row.CreatedAt.Time,
			CreatedBy: row.CreatedBy,
		})
	}
	return result, nil
}
