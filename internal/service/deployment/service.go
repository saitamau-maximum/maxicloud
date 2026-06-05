package deployment

import (
	"context"
	"fmt"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type DeploymentService interface {
	Deploy(ctx context.Context, spec domain.DeploymentSpec) (string, error)
	Retry(ctx context.Context, deploymentID string) (*domain.Deployment, error)
}

type service struct {
	history  *history
	workflow domain.DeploymentWorkflowRepository
}

func NewDeploymentService(
	historyRepo domain.DeploymentHistoryRepository,
	workflowRepo domain.DeploymentWorkflowRepository,
) DeploymentService {
	return &service{
		history:  &history{repo: historyRepo},
		workflow: workflowRepo,
	}
}

func (s *service) Deploy(ctx context.Context, spec domain.DeploymentSpec) (string, error) {
	record, err := s.history.Record(ctx, spec)
	if err != nil {
		return "", err
	}

	if _, err := s.workflow.Create(ctx, *record); err != nil {
		if markErr := s.history.MarkFailed(ctx, record.ID); markErr != nil {
			return "", fmt.Errorf("create workflow: %w (mark failed: %v)", err, markErr)
		}
		return "", fmt.Errorf("create workflow: %w", err)
	}

	_ = s.cleanPreviousWorkflows(ctx, spec.ApplicationID, spec.IsPreview())

	return record.ID, nil
}

func (s *service) Retry(ctx context.Context, deploymentID string) (*domain.Deployment, error) {
	current, err := s.history.Get(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get deployment history: %w", err)
	}
	if current == nil {
		return nil, domain.ValidationError{Message: "deployment not found"}
	}

	newID, err := s.Deploy(ctx, current.Spec)
	if err != nil {
		return nil, fmt.Errorf("retry deployment: %w", err)
	}
	return s.history.Get(ctx, newID)
}

const (
	maxPreviewDeployHistory    = 1
	maxProductionDeployHistory = 3
)

func (s *service) cleanPreviousWorkflows(ctx context.Context, applicationID string, isPreview bool) error {
	max := maxProductionDeployHistory
	if isPreview {
		max = maxPreviewDeployHistory
	}
	return s.workflow.Delete(ctx, applicationID, max, isPreview)
}
