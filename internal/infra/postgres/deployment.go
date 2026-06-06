package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/infra/postgres/db"
)

type deploymentHistoryRepository struct {
	q *db.Queries
}

var _ domain.DeploymentHistoryRepository = (*deploymentHistoryRepository)(nil)

func NewDeploymentHistoryRepository(pool *pgxpool.Pool) domain.DeploymentHistoryRepository {
	return &deploymentHistoryRepository{q: db.New(pool)}
}

func (r *deploymentHistoryRepository) Create(ctx context.Context, d domain.Deployment) (string, error) {
	var prNumber *int32
	if d.Spec.IsPreview() {
		v := int32(*d.Spec.PRNumber)
		prNumber = &v
	}
	return r.q.CreateDeploymentHistory(ctx, db.CreateDeploymentHistoryParams{
		ID:            d.ID,
		ApplicationID: d.Spec.ApplicationID,
		OwnerUserID:   d.Spec.OwnerUserID,
		RepoOwner:     d.Spec.Repo.Owner,
		RepoName:      d.Spec.Repo.Name,
		CommitSha:     d.Spec.Commit.SHA,
		CommitMessage: d.Spec.Commit.Message,
		CommitAuthor:  d.Spec.Commit.AuthorName,
		CommitAt:      pgtype.Timestamptz{Time: d.Spec.Commit.Timestamp, Valid: true},
		PrNumber:      prNumber,
		Status:        string(d.Status),
		StartedAt:     pgtype.Timestamptz{Time: d.StartedAt, Valid: true},
	})
}

func (r *deploymentHistoryRepository) Get(ctx context.Context, id string) (*domain.Deployment, error) {
	row, err := r.q.GetDeploymentHistory(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rowToDomain(row), nil
}

func (r *deploymentHistoryRepository) RecordStatus(ctx context.Context, params domain.RecordStatusParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	var finishedAt pgtype.Timestamptz
	if params.FinishedAt != nil {
		finishedAt = pgtype.Timestamptz{Time: *params.FinishedAt, Valid: true}
	}
	return r.q.UpdateDeploymentHistoryStatus(ctx, db.UpdateDeploymentHistoryStatusParams{
		ID:         params.ID,
		Status:     string(params.Status),
		FinishedAt: finishedAt,
	})
}

func (r *deploymentHistoryRepository) Delete(ctx context.Context, id string) error {
	return r.q.DeleteDeploymentHistory(ctx, id)
}

func (r *deploymentHistoryRepository) ListByApplication(ctx context.Context, applicationID string) ([]domain.Deployment, error) {
	rows, err := r.q.ListDeploymentHistoriesByApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Deployment, len(rows))
	for i, row := range rows {
		result[i] = *rowToDomain(row)
	}
	return result, nil
}

func rowToDomain(row db.DeploymentHistory) *domain.Deployment {
	var prNumber *int
	if row.PrNumber != nil {
		v := int(*row.PrNumber)
		prNumber = &v
	}
	var finishedAt *time.Time
	if row.FinishedAt.Valid {
		t := row.FinishedAt.Time
		finishedAt = &t
	}
	return &domain.Deployment{
		ID: row.ID,
		Spec: domain.DeploymentSpec{
			ApplicationID: row.ApplicationID,
			OwnerUserID:   row.OwnerUserID,
			Repo: domain.Repository{
				Owner: row.RepoOwner,
				Name:  row.RepoName,
			},
			Commit: domain.Commit{
				SHA:        row.CommitSha,
				Message:    row.CommitMessage,
				AuthorName: row.CommitAuthor,
				Timestamp:  row.CommitAt.Time,
			},
			PRNumber: prNumber,
		},
		Status:     domain.DeploymentStatus(row.Status),
		StartedAt:  row.StartedAt.Time,
		FinishedAt: finishedAt,
	}
}
