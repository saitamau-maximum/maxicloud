package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/infra/postgres/db"
)

type projectMemberRepository struct {
	q *db.Queries
}

var _ domain.ProjectMemberRepository = (*projectMemberRepository)(nil)

func NewProjectMemberRepository(pool *pgxpool.Pool) domain.ProjectMemberRepository {
	return &projectMemberRepository{q: db.New(pool)}
}

func (r *projectMemberRepository) GetByUser(ctx context.Context, projectID, userID string) (*domain.ProjectMember, error) {
	row, err := r.q.GetProjectMember(ctx, db.GetProjectMemberParams{
		ProjectID: projectID,
		UserID:    userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get project member: %w", err)
	}
	m, err := memberRowToDomain(row)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *projectMemberRepository) ListByProject(ctx context.Context, projectID string) ([]domain.ProjectMember, error) {
	rows, err := r.q.ListProjectMembers(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}
	members := make([]domain.ProjectMember, 0, len(rows))
	for _, row := range rows {
		m, err := memberRowToDomain(row)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *projectMemberRepository) Add(ctx context.Context, member domain.ProjectMember) error {
	_, err := r.q.AddProjectMember(ctx, db.AddProjectMemberParams{
		ID:        member.ID,
		ProjectID: member.ProjectID,
		UserID:    member.UserID,
		Role:      member.Role.String(),
		CreatedBy: member.CreatedBy,
	})
	return err
}

func (r *projectMemberRepository) Remove(ctx context.Context, id string) error {
	return r.q.RemoveProjectMember(ctx, id)
}

func (r *projectMemberRepository) UpdateRole(ctx context.Context, id string, role domain.Role) error {
	return r.q.UpdateProjectMemberRole(ctx, db.UpdateProjectMemberRoleParams{
		ID:   id,
		Role: role.String(),
	})
}

func memberRowToDomain(row db.ProjectMember) (domain.ProjectMember, error) {
	role, err := domain.ParseRole(row.Role)
	if err != nil {
		return domain.ProjectMember{}, fmt.Errorf("parse role: %w", err)
	}
	return domain.ProjectMember{
		ID:        row.ID,
		ProjectID: row.ProjectID,
		UserID:    row.UserID,
		Role:      role,
		CreatedAt: row.CreatedAt.Time,
		CreatedBy: row.CreatedBy,
	}, nil
}
