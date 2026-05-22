package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type stubApplicationRepo struct {
	created *domain.Application
	apps    map[string]*domain.Application
}

func newStubApplicationRepo() *stubApplicationRepo {
	return &stubApplicationRepo{apps: map[string]*domain.Application{}}
}

func (s *stubApplicationRepo) CreateApplication(ctx context.Context, params domain.CreateApplicationParams) (*domain.Application, error) {
	app := &domain.Application{
		ID:        params.ID,
		ProjectID: params.Spec.ProjectID,
		Name:      params.Name,
		Source:    params.Spec.Source,
		OwnerID:   params.OwnerID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.created = app
	s.apps[app.ID] = app
	return app, nil
}

func (s *stubApplicationRepo) GetApplication(ctx context.Context, id string) (*domain.Application, error) {
	return s.apps[id], nil
}

func (s *stubApplicationRepo) ListApplications(ctx context.Context, projectID string) ([]domain.Application, error) {
	return nil, nil
}

func (s *stubApplicationRepo) UpdateApplication(ctx context.Context, app domain.Application) error {
	s.apps[app.ID] = &app
	return nil
}

func (s *stubApplicationRepo) DeleteApplication(ctx context.Context, id string) error {
	delete(s.apps, id)
	return nil
}

func (s *stubApplicationRepo) GetApplicationsByRepo(ctx context.Context, owner, name, branch string) ([]domain.Application, error) {
	return nil, nil
}

func (s *stubApplicationRepo) ExistsByDomain(ctx context.Context, fqdn string) (bool, error) {
	return false, nil
}

func (s *stubApplicationRepo) CreatePreviewApplication(ctx context.Context, originalApplicationID string, prNumber int) (*domain.Application, error) {
	return nil, nil
}

type stubSourceService struct {
	commit domain.Commit
	err    error
}

func (s *stubSourceService) GetRepositories(ctx context.Context) ([]domain.Repository, error) {
	return nil, nil
}

func (s *stubSourceService) GetBranches(ctx context.Context, repo domain.Repository) ([]string, error) {
	return nil, nil
}

func (s *stubSourceService) GetHeadCommit(ctx context.Context, repo domain.Repository, branch string) (domain.Commit, error) {
	if s.err != nil {
		return domain.Commit{}, s.err
	}
	return s.commit, nil
}

type stubDeployCreator struct {
	id         string
	err        error
	called     bool
	lastParams CreateDeploymentParams
}

func (s *stubDeployCreator) CreateDeployment(ctx context.Context, params CreateDeploymentParams) (string, error) {
	s.called = true
	s.lastParams = params
	if s.err != nil {
		return "", s.err
	}
	return s.id, nil
}

func validCreateSpec() domain.ApplicationSpec {
	return domain.ApplicationSpec{
		ProjectID: "550e8400-e29b-41d4-a716-446655440000",
		Source: domain.ApplicationSource{
			Repo:   domain.Repository{Owner: "octocat", Name: "hello-world"},
			Branch: "main",
		},
		BuildConfig: domain.BuildConfigDockerfile{Source: domain.DockerfileSourcePath{Path: "Dockerfile"}},
		AccessMode:  domain.AccessModePublic,
		Domain:      &domain.Domain{Subdomain: "demo", RootDomain: "apps.maximum.vc"},
		Port:        3000,
	}
}

func TestCreateApplication_StartInitialDeployment(t *testing.T) {
	repo := newStubApplicationRepo()
	sourceSvc := &stubSourceService{commit: domain.Commit{SHA: "abc123", Message: "init", AuthorName: "kouta", Timestamp: time.Now()}}
	deploy := &stubDeployCreator{id: "deploy-1"}
	svc := NewApplicationService(repo, deploy, sourceSvc)

	result, err := svc.CreateApplication(context.Background(), CreateApplicationParams{
		Name:    "app",
		OwnerID: "11111111-1111-1111-1111-111111111111",
		Spec:    validCreateSpec(),
	})
	if err != nil {
		t.Fatalf("CreateApplication returned error: %v", err)
	}
	if result == nil || result.Application == nil {
		t.Fatalf("expected application in result")
	}
	if !result.InitialDeploymentStarted {
		t.Fatalf("expected initial deployment started")
	}
	if result.InitialDeploymentID != "deploy-1" {
		t.Fatalf("expected deployment id deploy-1, got %q", result.InitialDeploymentID)
	}
	if !deploy.called {
		t.Fatalf("expected deployment to be created")
	}
	if deploy.lastParams.ApplicationID != result.Application.ID {
		t.Fatalf("expected deployment app id %q, got %q", result.Application.ID, deploy.lastParams.ApplicationID)
	}
}

func TestCreateApplication_HeadCommitFailureDoesNotFailApplicationCreation(t *testing.T) {
	repo := newStubApplicationRepo()
	sourceSvc := &stubSourceService{err: errors.New("github unavailable")}
	deploy := &stubDeployCreator{id: "deploy-1"}
	svc := NewApplicationService(repo, deploy, sourceSvc)

	result, err := svc.CreateApplication(context.Background(), CreateApplicationParams{
		Name:    "app",
		OwnerID: "11111111-1111-1111-1111-111111111111",
		Spec:    validCreateSpec(),
	})
	if err != nil {
		t.Fatalf("CreateApplication returned error: %v", err)
	}
	if result == nil || result.Application == nil {
		t.Fatalf("expected application in result")
	}
	if result.InitialDeploymentStarted {
		t.Fatalf("expected initial deployment not started")
	}
	if result.InitialDeploymentID != "" {
		t.Fatalf("expected empty deployment id")
	}
	if !strings.Contains(result.InitialDeploymentError, "failed to resolve HEAD commit") {
		t.Fatalf("unexpected deployment error: %q", result.InitialDeploymentError)
	}
	if deploy.called {
		t.Fatalf("expected deployment create not called")
	}
}

func TestCreateApplication_DeploymentFailureDoesNotFailApplicationCreation(t *testing.T) {
	repo := newStubApplicationRepo()
	sourceSvc := &stubSourceService{commit: domain.Commit{SHA: "abc123", Message: "init", AuthorName: "kouta", Timestamp: time.Now()}}
	deploy := &stubDeployCreator{err: errors.New("pipeline repo failed")}
	svc := NewApplicationService(repo, deploy, sourceSvc)

	result, err := svc.CreateApplication(context.Background(), CreateApplicationParams{
		Name:    "app",
		OwnerID: "11111111-1111-1111-1111-111111111111",
		Spec:    validCreateSpec(),
	})
	if err != nil {
		t.Fatalf("CreateApplication returned error: %v", err)
	}
	if result == nil || result.Application == nil {
		t.Fatalf("expected application in result")
	}
	if result.InitialDeploymentStarted {
		t.Fatalf("expected initial deployment not started")
	}
	if result.InitialDeploymentID != "" {
		t.Fatalf("expected empty deployment id")
	}
	if !strings.Contains(result.InitialDeploymentError, "failed to create initial deployment") {
		t.Fatalf("unexpected deployment error: %q", result.InitialDeploymentError)
	}
	if !deploy.called {
		t.Fatalf("expected deployment create called")
	}
}
