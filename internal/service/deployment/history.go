package deployment

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type History interface {
	Get(ctx context.Context, deploymentID string) (*domain.Deployment, error)
	List(ctx context.Context, applicationID string) ([]domain.Deployment, error)
}

type history struct {
	repo domain.DeploymentHistoryRepository
}

func NewHistory(repo domain.DeploymentHistoryRepository) History {
	return &history{repo: repo}
}

func (h *history) Record(ctx context.Context, spec domain.DeploymentSpec) (*domain.Deployment, error) {
	deployment := domain.Deployment{
		ID:        uuid.New().String(),
		Spec:      spec,
		Status:    domain.DeploymentStatusQueued,
		StartedAt: time.Now(),
	}
	id, err := h.repo.Create(ctx, deployment)
	if err != nil {
		return nil, err
	}
	deployment.ID = id
	return &deployment, nil
}

func (h *history) MarkFailed(ctx context.Context, id string) error {
	now := time.Now()
	return h.repo.RecordStatus(ctx, domain.RecordStatusParams{
		ID:         id,
		Status:     domain.DeploymentStatusFailed,
		FinishedAt: &now,
	})
}

func (h *history) Get(ctx context.Context, deploymentID string) (*domain.Deployment, error) {
	return h.repo.Get(ctx, deploymentID)
}

func (h *history) List(ctx context.Context, applicationID string) ([]domain.Deployment, error) {
	if applicationID == "" {
		return nil, domain.ValidationError{Message: "application_id is required"}
	}
	return h.repo.ListByApplication(ctx, applicationID)
}
