package deployment

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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
	apps, err := s.appRepo.ListByRepo(ctx, event.Repo.Owner, event.Repo.Name, event.Branch)
	if err != nil {
		return fmt.Errorf("get applications by repo: %w", err)
	}

	for _, app := range apps {
		if prNumber == nil {
			if _, err := s.deploySvc.Deploy(ctx, domain.DeploymentSpec{
				ApplicationID: app.ID,
				OwnerUserID:   app.OwnerID,
				Repo:          event.Repo,
				Commit:        event.Commit,
				PRNumber:      prNumber,
			}); err != nil {
				return fmt.Errorf("create deployment for application %s: %w", app.ID, err)
			}
			continue
		}

		previewApp, err := s.appRepo.CreatePreviewApplication(ctx, app.ID, *prNumber, uuid.New().String())
		if err != nil {
			return fmt.Errorf("create preview application for %s: %w", app.ID, err)
		}
		if _, err := s.deploySvc.Deploy(ctx, domain.DeploymentSpec{
			ApplicationID: previewApp.ID,
			OwnerUserID:   previewApp.OwnerID,
			Repo:          event.Repo,
			Commit:        event.Commit,
			PRNumber:      prNumber,
		}); err != nil {
			return fmt.Errorf("create deployment for preview application %s: %w", previewApp.ID, err)
		}
	}
	return nil
}
