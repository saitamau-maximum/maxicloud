package deployment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type DeploymentEventService interface {
	HandleDeploymentEvent(ctx context.Context, event domain.DeploymentEvent) error
}

type eventService struct {
	appRepo     domain.ApplicationRepository
	projectRepo domain.ProjectRepository
	deploySvc   DeploymentService
}

func NewDeploymentEventService(appRepo domain.ApplicationRepository, projectRepo domain.ProjectRepository, deploySvc DeploymentService) DeploymentEventService {
	return &eventService{
		appRepo:     appRepo,
		projectRepo: projectRepo,
		deploySvc:   deploySvc,
	}
}

func (s *eventService) HandleDeploymentEvent(ctx context.Context, event domain.DeploymentEvent) error {
	switch event.Type {
	case domain.DeploymentEventTypeProductionRequested:
		return s.deployMatchingApplications(ctx, event, nil)
	case domain.DeploymentEventTypePreviewRequested:
		if event.PRNumber == nil {
			return domain.ValidationError{Message: "PR number is required for preview deployment"}
		}
		return s.deployMatchingApplications(ctx, event, event.PRNumber)
	case domain.DeploymentEventTypePreviewDeleted:
		// TODO: いつか実装する
		return nil
	default:
		return domain.ValidationError{Message: fmt.Sprintf("unsupported deployment event type: %s", event.Type)}
	}
}

func (s *eventService) deployMatchingApplications(ctx context.Context, event domain.DeploymentEvent, prNumber *int) error {
	apps, err := s.appRepo.ListByRepo(ctx, event.Repo.Owner, event.Repo.Name, event.Branch)
	if err != nil {
		return fmt.Errorf("get applications by repo: %w", err)
	}

	for _, app := range apps {
		if prNumber == nil {
			if err := s.deploy(ctx, event, app); err != nil {
				return err
			}
			continue
		}
		if err := s.deployPreview(ctx, event, app, *prNumber); err != nil {
			return err
		}
	}
	return nil
}

func (s *eventService) deploy(ctx context.Context, event domain.DeploymentEvent, app domain.Application) error {
	if _, err := s.deploySvc.Deploy(ctx, domain.DeploymentSpec{
		ApplicationID: app.ID,
		OwnerUserID:   app.OwnerID,
		Repo:          event.Repo,
		Commit:        event.Commit,
	}); err != nil {
		return fmt.Errorf("deploy application %s: %w", app.ID, err)
	}
	return nil
}

func (s *eventService) deployPreview(ctx context.Context, event domain.DeploymentEvent, app domain.Application, prNumber int) error {
	now := time.Now()
	previewProject, err := s.projectRepo.CreatePreview(ctx, domain.CreatePreviewParams{
		ID:                uuid.New().String(),
		OriginalProjectID: app.Spec.ProjectID,
		OwnerID:           app.OwnerID,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		return fmt.Errorf("create preview project for %s: %w", app.ID, err)
	}

	previewSpec := app.Spec
	previewSpec.ProjectID = previewProject.ID
	previewSpec.Source.Branch = domain.PreviewBranch(prNumber)
	if previewSpec.Domain != nil {
		previewDomain := previewSpec.Domain.Preview(prNumber)
		previewSpec.Domain = &previewDomain
	}

	preview, err := s.appRepo.CreatePreview(ctx, domain.CreatePreviewApplicationParams{
		ID:                    uuid.New().String(),
		ProjectID:             previewProject.ID,
		Name:                  domain.PreviewName(app.Name, prNumber),
		OwnerID:               app.OwnerID,
		OriginalApplicationID: app.ID,
		PRNumber:              prNumber,
		Spec:                  previewSpec,
	})
	if err != nil {
		return fmt.Errorf("create preview application for %s: %w", app.ID, err)
	}
	if _, err := s.deploySvc.Deploy(ctx, domain.DeploymentSpec{
		ApplicationID: preview.ID,
		OwnerUserID:   preview.OwnerID,
		Repo:          event.Repo,
		Commit:        event.Commit,
		PRNumber:      &prNumber,
	}); err != nil {
		return fmt.Errorf("deploy preview application %s: %w", preview.ID, err)
	}
	return nil
}
