package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/usecase/deployment"
)

type ApplicationService interface {
	CreateApplication(ctx context.Context, params CreateApplicationParams) (*CreateApplicationResult, error)
	GetApplication(ctx context.Context, id string) (*domain.Application, error)
	ListApplications(ctx context.Context, projectID string) ([]domain.Application, error)
	UpdateApplication(ctx context.Context, params UpdateApplicationParams) (*domain.Application, error)
	DeleteApplication(ctx context.Context, id string) error
}

type applicationService struct {
	appRepo   domain.ApplicationRepository
	deploySvc deployment.DeploymentService
	sourceSvc SourceService
}

func NewApplicationService(
	appRepo domain.ApplicationRepository,
	deploySvc deployment.DeploymentService,
	sourceSvc SourceService,
) ApplicationService {
	return &applicationService{
		appRepo:   appRepo,
		deploySvc: deploySvc,
		sourceSvc: sourceSvc,
	}
}

type CreateApplicationParams struct {
	Name    string
	OwnerID string
	Spec    domain.ApplicationSpec
}

type CreateApplicationResult struct {
	Application              *domain.Application
	InitialDeploymentID      string
	InitialDeploymentStarted bool
	InitialDeploymentError   string
}

func (u *applicationService) CreateApplication(ctx context.Context, params CreateApplicationParams) (*CreateApplicationResult, error) {
	if err := params.Spec.Validate(); err != nil {
		return nil, err
	}
	createdApp, err := u.appRepo.CreateApplication(ctx, domain.CreateApplicationParams{
		ID:      uuid.New().String(),
		Name:    params.Name,
		OwnerID: params.OwnerID,
		Spec:    params.Spec,
	})
	if err != nil {
		return nil, err
	}
	app, err := u.appRepo.GetApplication(ctx, createdApp.ID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, fmt.Errorf("created application not found: %s", createdApp.ID)
	}

	result := &CreateApplicationResult{
		Application: app,
	}

	headCommit, err := u.sourceSvc.GetHeadCommit(ctx, app.Source.Repo, app.Source.Branch)
	if err != nil {
		result.InitialDeploymentError = fmt.Sprintf("failed to resolve HEAD commit: %v", err)
		return result, nil
	}

	deployID, err := u.deploySvc.Deploy(ctx, domain.DeploymentSpec{
		ApplicationID: app.ID,
		OwnerUserID:   app.OwnerID,
		Repo:          app.Source.Repo,
		Commit:        headCommit,
	})
	if err != nil {
		result.InitialDeploymentError = fmt.Sprintf("failed to create initial deployment: %v", err)
		return result, nil
	}

	result.InitialDeploymentStarted = true
	result.InitialDeploymentID = deployID
	return result, nil
}

func (u *applicationService) GetApplication(ctx context.Context, id string) (*domain.Application, error) {
	return u.appRepo.GetApplication(ctx, id)
}

func (u *applicationService) ListApplications(ctx context.Context, projectID string) ([]domain.Application, error) {
	return u.appRepo.ListApplications(ctx, projectID)
}

type UpdateApplicationParams struct {
	ID      string
	Name    string
	OwnerID string
	Spec    domain.ApplicationSpec
}

func (u *applicationService) UpdateApplication(ctx context.Context, params UpdateApplicationParams) (*domain.Application, error) {
	if err := params.Spec.Validate(); err != nil {
		return nil, err
	}
	existing, err := u.appRepo.GetApplication(ctx, params.ID)
	if err != nil {
		return nil, err
	}
	existing.ProjectID = params.Spec.ProjectID
	existing.Name = params.Name
	existing.OwnerID = params.OwnerID
	existing.Source = params.Spec.Source
	if err := u.appRepo.UpdateApplication(ctx, *existing); err != nil {
		return nil, err
	}
	return u.appRepo.GetApplication(ctx, params.ID)
}

func (u *applicationService) DeleteApplication(ctx context.Context, id string) error {
	return u.appRepo.DeleteApplication(ctx, id)
}
