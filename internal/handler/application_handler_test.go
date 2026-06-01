package handler

import (
	"context"
	"testing"
	"time"

	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type stubApplicationService struct {
	createResult *service.CreateApplicationResult
}

func (s *stubApplicationService) Create(ctx context.Context, params service.CreateApplicationParams) (*service.CreateApplicationResult, error) {
	return s.createResult, nil
}

func (s *stubApplicationService) Get(ctx context.Context, id string) (*domain.Application, error) {
	return nil, nil
}

func (s *stubApplicationService) List(ctx context.Context, projectID string) ([]domain.Application, error) {
	return nil, nil
}

func (s *stubApplicationService) Update(ctx context.Context, params service.UpdateApplicationParams) (*domain.Application, error) {
	return nil, nil
}

func (s *stubApplicationService) Delete(ctx context.Context, id string) error {
	return nil
}

func TestCreateApplicationResponseIncludesInitialDeploymentMeta(t *testing.T) {
	now := time.Now()
	svc := &stubApplicationService{
		createResult: &service.CreateApplicationResult{
			Application: &domain.Application{
				ID:        "app-1",
				ProjectID: "550e8400-e29b-41d4-a716-446655440000",
				Name:      "sample",
				Source: domain.ApplicationSource{
					Repo:   domain.Repository{Owner: "octocat", Name: "hello-world"},
					Branch: "main",
				},
				OwnerID:   "11111111-1111-1111-1111-111111111111",
				CreatedAt: now,
				UpdatedAt: now,
			},
			InitialDeploymentID:      "deploy-1",
			InitialDeploymentStarted: true,
			InitialDeploymentError:   "",
		},
	}
	h := NewApplicationHandler(svc)

	res, err := h.CreateApplication(context.Background(), &v1.CreateApplicationRequest{
		Name:    "sample",
		OwnerId: "11111111-1111-1111-1111-111111111111",
		Spec: &v1.ApplicationSpec{
			ProjectId: "550e8400-e29b-41d4-a716-446655440000",
			Source: &v1.ApplicationSource{
				Repository: &v1.Repository{Owner: "octocat", Name: "hello-world"},
				Branch:     "main",
			},
			Build: &v1.BuildConfig{
				Strategy: v1.BuildStrategy_BUILD_STRATEGY_DOCKERFILE,
				Dockerfile: &v1.DockerfileBuildConfig{
					Source:         v1.DockerfileSource_DOCKERFILE_SOURCE_PATH,
					DockerfilePath: "Dockerfile",
				},
			},
			Access: &v1.Access{
				Mode: v1.AccessMode_ACCESS_MODE_PUBLIC,
				Domain: &v1.Domain{
					Subdomain:  "demo",
					RootDomain: "apps.maximum.vc",
				},
				Port: 3000,
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateApplication returned error: %v", err)
	}
	if res.GetApplication().GetId() != "app-1" {
		t.Fatalf("expected app id app-1, got %q", res.GetApplication().GetId())
	}
	if res.GetInitialDeploymentId() != "deploy-1" {
		t.Fatalf("expected initial deployment id deploy-1, got %q", res.GetInitialDeploymentId())
	}
	if !res.GetInitialDeploymentStarted() {
		t.Fatalf("expected initial deployment started")
	}
	if res.GetInitialDeploymentError() != "" {
		t.Fatalf("expected empty initial deployment error, got %q", res.GetInitialDeploymentError())
	}

	if got := res.GetApplication().GetCreatedAt(); got == nil || got.AsTime().IsZero() {
		t.Fatalf("expected created_at set")
	}
	if got := res.GetApplication().GetUpdatedAt(); got == nil || got.AsTime().IsZero() {
		t.Fatalf("expected updated_at set")
	}
	_ = timestamppb.Now // keep protobuf timestamp import used by generated accessors consistency
}
