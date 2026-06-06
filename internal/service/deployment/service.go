package deployment

import (
	"context"
	"fmt"
	"log"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type DeploymentService interface {
	Deploy(ctx context.Context, spec domain.DeploymentSpec) (string, error)
	Retry(ctx context.Context, deploymentID string) (*domain.Deployment, error)
}

type service struct {
	history   *history
	deployRun domain.DeployRunRepository
}

func NewDeploymentService(
	historyRepo domain.DeploymentHistoryRepository,
	deployRunRepo domain.DeployRunRepository,
) DeploymentService {
	return &service{
		history:   &history{repo: historyRepo},
		deployRun: deployRunRepo,
	}
}

func (s *service) Deploy(ctx context.Context, spec domain.DeploymentSpec) (string, error) {
	record, err := s.history.Record(ctx, spec)
	if err != nil {
		return "", err
	}

	if _, err := s.deployRun.Create(ctx, *record); err != nil {
		if markErr := s.history.MarkFailed(ctx, record.ID); markErr != nil {
			return "", fmt.Errorf("create deploy run: %w (mark failed: %v)", err, markErr)
		}
		return "", fmt.Errorf("create deploy run: %w", err)
	}

	if err := s.cleanPreviousDeployRuns(ctx, spec.ApplicationID, spec.IsPreview()); err != nil {
		log.Printf(
			"deployment: failed to clean previous deploy runs application_id=%s preview=%t: %v",
			spec.ApplicationID,
			spec.IsPreview(),
			err,
		)
	}

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

func (s *service) cleanPreviousDeployRuns(ctx context.Context, applicationID string, isPreview bool) error {
	max := maxProductionDeployHistory
	if isPreview {
		max = maxPreviewDeployHistory
	}
	return s.deployRun.Delete(ctx, applicationID, max, isPreview)
}
