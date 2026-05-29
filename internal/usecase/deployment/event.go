package deployment

import (
	"context"
	"fmt"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type DeploymentEventService interface {
	HandleDeploymentEvent(ctx context.Context, event domain.DeploymentEvent) error
}

type eventService struct {
	appRepo   domain.ApplicationRepository
	deploySvc DeploymentService
}

func NewDeploymentEventService(appRepo domain.ApplicationRepository, deploySvc DeploymentService) DeploymentEventService {
	return &eventService{
		appRepo:   appRepo,
		deploySvc: deploySvc,
	}
}

func (s *eventService) HandleDeploymentEvent(ctx context.Context, event domain.DeploymentEvent) error {
	switch event.Type {
	case domain.DeploymentEventTypeProductionRequested:
		return s.handleRepoDeploymentEvent(ctx, event, nil)
	case domain.DeploymentEventTypePreviewRequested:
		if event.PRNumber == nil {
			return domain.ValidationError{Message: "PR number is required for preview deployment"}
		}
		return s.handleRepoDeploymentEvent(ctx, event, event.PRNumber)
	case domain.DeploymentEventTypePreviewDeleted:
		// TODO: いつか実装する
		return nil
	default:
		return domain.ValidationError{Message: fmt.Sprintf("unsupported deployment event type: %s", event.Type)}
	}
}

func (s *eventService) handleRepoDeploymentEvent(ctx context.Context, event domain.DeploymentEvent, prNumber *int) error {
	apps, err := s.appRepo.GetApplicationsByRepo(ctx, event.Repo.Owner, event.Repo.Name, event.Branch)
	if err != nil {
		return fmt.Errorf("get applications by repo: %w", err)
	}

	for _, app := range apps {
		spec := domain.DeploymentSpec{
			ApplicationID: app.ID,
			OwnerUserID:   app.OwnerID,
			Repo:          event.Repo,
			Commit:        event.Commit,
			PRNumber:      prNumber,
		}
		if _, err := s.deploySvc.Deploy(ctx, spec); err != nil {
			return fmt.Errorf("create deployment for application %s: %w", app.ID, err)
		}
	}
	return nil
}
