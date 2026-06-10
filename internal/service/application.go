package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/service/authz"
	"github.com/saitamau-maximum/maxicloud/internal/service/deployment"
)

type ApplicationService interface {
	Create(ctx context.Context, params CreateApplicationParams) (*CreateApplicationResult, error)
	Get(ctx context.Context, id string) (*domain.Application, error)
	List(ctx context.Context, projectID string) ([]domain.Application, error)
	Update(ctx context.Context, params UpdateApplicationParams) (*domain.Application, error)
	Delete(ctx context.Context, id string) error
}

type applicationService struct {
	appRepo     domain.ApplicationRepository
	projectRepo domain.ProjectRepository
	deploySvc   deployment.DeploymentService
	sourceSvc   SourceService
	authz       authz.Authorizer
}

func NewApplicationService(
	appRepo domain.ApplicationRepository,
	projectRepo domain.ProjectRepository,
	deploySvc deployment.DeploymentService,
	sourceSvc SourceService,
	authorizer authz.Authorizer,
) ApplicationService {
	return &applicationService{
		appRepo:     appRepo,
		projectRepo: projectRepo,
		deploySvc:   deploySvc,
		sourceSvc:   sourceSvc,
		authz:       authorizer,
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

func (u *applicationService) Create(ctx context.Context, params CreateApplicationParams) (*CreateApplicationResult, error) {
	if err := params.Spec.Validate(); err != nil {
		return nil, err
	}
	project, err := u.projectRepo.Get(ctx, params.Spec.ProjectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, domain.ValidationError{Message: "project not found"}
	}
	if err := u.authz.Authorize(ctx, project, domain.PermissionWriteApplication); err != nil {
		return nil, err
	}
	createdApp, err := u.appRepo.Create(ctx, domain.CreateApplicationParams{
		ID:      uuid.New().String(),
		Name:    params.Name,
		OwnerID: params.OwnerID,
		Spec:    params.Spec,
	})
	if err != nil {
		return nil, err
	}
	app, err := u.appRepo.Get(ctx, createdApp.ID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, fmt.Errorf("created application not found: %s", createdApp.ID)
	}

	result := &CreateApplicationResult{
		Application: app,
	}

	headCommit, err := u.sourceSvc.GetHeadCommit(ctx, app.Spec.Source.Repo, app.Spec.Source.Branch)
	if err != nil {
		result.InitialDeploymentError = fmt.Sprintf("failed to resolve HEAD commit: %v", err)
		return result, nil
	}

	deployID, err := u.deploySvc.Deploy(ctx, domain.DeploymentSpec{
		ApplicationID: app.ID,
		OwnerUserID:   app.OwnerID,
		Repo:          app.Spec.Source.Repo,
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

func (u *applicationService) Get(ctx context.Context, id string) (*domain.Application, error) {
	return u.appRepo.Get(ctx, id)
}

func (u *applicationService) List(ctx context.Context, projectID string) ([]domain.Application, error) {
	return u.appRepo.List(ctx, projectID)
}

type UpdateApplicationParams struct {
	ID      string
	Name    string
	OwnerID string
	Spec    domain.ApplicationSpec
}

func (u *applicationService) Update(ctx context.Context, params UpdateApplicationParams) (*domain.Application, error) {
	if err := params.Spec.Validate(); err != nil {
		return nil, err
	}
	project, err := u.projectRepo.Get(ctx, params.Spec.ProjectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, domain.ValidationError{Message: "project not found"}
	}
	if err := u.authz.Authorize(ctx, project, domain.PermissionWriteApplication); err != nil {
		return nil, err
	}
	if err := u.appRepo.Update(ctx, domain.UpdateApplicationParams{
		ID:      params.ID,
		Name:    params.Name,
		OwnerID: params.OwnerID,
		Spec:    params.Spec,
	}); err != nil {
		return nil, err
	}
	return u.appRepo.Get(ctx, params.ID)
}

func (u *applicationService) Delete(ctx context.Context, id string) error {
	app, err := u.appRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	if app == nil {
		return nil
	}
	project, err := u.projectRepo.Get(ctx, app.Spec.ProjectID)
	if err != nil {
		return err
	}
	if project == nil {
		return domain.ValidationError{Message: "project not found"}
	}
	if err := u.authz.Authorize(ctx, project, domain.PermissionDeleteApplication); err != nil {
		return err
	}
	return u.appRepo.Delete(ctx, id)
}
