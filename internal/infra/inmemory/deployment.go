package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type deploymentHistoryRepository struct {
	mu   sync.RWMutex
	data map[string]*domain.Deployment
}

func NewDeploymentHistoryRepository() domain.DeploymentHistoryRepository {
	return &deploymentHistoryRepository{data: make(map[string]*domain.Deployment)}
}

func (r *deploymentHistoryRepository) Create(_ context.Context, deployment domain.Deployment) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := uuid.NewString()
	d := deployment
	d.ID = id
	if d.StartedAt.IsZero() {
		d.StartedAt = time.Now()
	}
	r.data[id] = &d
	return id, nil
}

func (r *deploymentHistoryRepository) Get(_ context.Context, id string) (*domain.Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.data[id]
	if !ok {
		return nil, nil
	}
	cp := *d
	return &cp, nil
}

func (r *deploymentHistoryRepository) RecordStatus(_ context.Context, params domain.RecordStatusParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.data[params.ID]
	if !ok {
		return domain.ValidationError{Message: "deployment not found"}
	}
	d.Status = params.Status
	d.FinishedAt = params.FinishedAt
	return nil
}

func (r *deploymentHistoryRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, id)
	return nil
}

func (r *deploymentHistoryRepository) ListByApplication(_ context.Context, applicationID string) ([]domain.Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []domain.Deployment
	for _, d := range r.data {
		if d.Spec.ApplicationID == applicationID {
			result = append(result, *d)
		}
	}
	return result, nil
}
