package handler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1/maxicloudv1connect"
	"github.com/saitamau-maximum/maxicloud/internal/auth"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ApplicationHandler struct {
	maxicloudv1connect.UnimplementedApplicationServiceHandler
	service service.ApplicationService
}

func NewApplicationHandler(svc service.ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{service: svc}
}

func (h *ApplicationHandler) CreateApplication(ctx context.Context, req *v1.CreateApplicationRequest) (*v1.CreateApplicationResponse, error) {
	spec, err := toApplicationSpec(req.Spec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.service.Create(ctx, service.CreateApplicationParams{
		Name:    req.Name,
		OwnerID: auth.UserID(ctx),
		Spec:    spec,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &v1.CreateApplicationResponse{
		Application:              toProtoApplication(result.Application),
		InitialDeploymentId:      optionalString(result.InitialDeploymentID),
		InitialDeploymentStarted: result.InitialDeploymentStarted,
		InitialDeploymentError:   optionalString(result.InitialDeploymentError),
	}, nil
}

func (h *ApplicationHandler) GetApplication(ctx context.Context, req *v1.GetApplicationRequest) (*v1.GetApplicationResponse, error) {
	app, err := h.service.Get(ctx, req.ApplicationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if app == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return &v1.GetApplicationResponse{Application: toProtoApplication(app)}, nil
}

func (h *ApplicationHandler) ListApplications(ctx context.Context, req *v1.ListApplicationsRequest) (*v1.ListApplicationsResponse, error) {
	apps, err := h.service.List(ctx, req.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var protoApps []*v1.Application
	for i := range apps {
		protoApps = append(protoApps, toProtoApplication(&apps[i]))
	}
	return &v1.ListApplicationsResponse{Applications: protoApps}, nil
}

func (h *ApplicationHandler) UpdateApplication(ctx context.Context, req *v1.UpdateApplicationRequest) (*v1.UpdateApplicationResponse, error) {
	spec, err := toApplicationSpec(req.Spec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	app, err := h.service.Update(ctx, service.UpdateApplicationParams{
		ID:      req.ApplicationId,
		Name:    req.Name,
		OwnerID: req.OwnerId,
		Spec:    spec,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &v1.UpdateApplicationResponse{Application: toProtoApplication(app)}, nil
}

func (h *ApplicationHandler) DeleteApplication(ctx context.Context, req *v1.DeleteApplicationRequest) (*v1.DeleteApplicationResponse, error) {
	if err := h.service.Delete(ctx, req.ApplicationId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &v1.DeleteApplicationResponse{}, nil
}

func toProtoApplication(a *domain.Application) *v1.Application {
	var protoDomain *v1.Domain
	if a.Condition.Domain != nil {
		protoDomain = &v1.Domain{
			Subdomain:  a.Condition.Domain.Subdomain,
			RootDomain: a.Condition.Domain.RootDomain,
		}
	}
	return &v1.Application{
		Id:          a.ID,
		ProjectId:   a.Spec.ProjectID,
		Name:        a.Name,
		OwnerUserId: a.OwnerID,
		Source: &v1.ApplicationSource{
			Repository: &v1.Repository{
				Name:  a.Spec.Source.Repo.Name,
				Owner: a.Spec.Source.Repo.Owner,
			},
			Branch: a.Spec.Source.Branch,
		},
		Condition: &v1.ApplicationCondition{
			Status: toProtoAppStatus(a.Condition.Status),
			Domain: protoDomain,
		},
		CreatedAt: timestamppb.New(a.CreatedAt),
		UpdatedAt: timestamppb.New(a.UpdatedAt),
	}
}

func toProtoAppStatus(s domain.ApplicationStatus) v1.ApplicationStatus {
	switch s {
	case domain.ApplicationStatusRunning:
		return v1.ApplicationStatus_APPLICATION_STATUS_RUNNING
	case domain.ApplicationStatusUnavailable:
		return v1.ApplicationStatus_APPLICATION_STATUS_UNAVAILABLE
	case domain.ApplicationStatusStopped:
		return v1.ApplicationStatus_APPLICATION_STATUS_STOPPED
	default:
		return v1.ApplicationStatus_APPLICATION_STATUS_UNSPECIFIED
	}
}

func toApplicationSpec(s *v1.ApplicationSpec) (domain.ApplicationSpec, error) {
	if s == nil {
		return domain.ApplicationSpec{}, nil
	}
	spec := domain.ApplicationSpec{
		ProjectID: s.ProjectId,
		Port:      s.GetAccess().GetPort(),
	}
	if s.Source != nil {
		spec.Source = domain.ApplicationSource{
			Repo: domain.Repository{
				Owner: s.Source.GetRepository().GetOwner(),
				Name:  s.Source.GetRepository().GetName(),
			},
			Branch: s.Source.Branch,
		}
	}
	if s.GetBuild() != nil {
		switch s.GetBuild().GetStrategy() {
		case v1.BuildStrategy_BUILD_STRATEGY_DOCKERFILE:
			if s.GetBuild().GetDockerfile() == nil {
				return domain.ApplicationSpec{}, errors.New("dockerfile build config is required")
			}
			switch s.GetBuild().GetDockerfile().GetSource() {
			case v1.DockerfileSource_DOCKERFILE_SOURCE_PATH:
				spec.BuildConfig = domain.BuildConfigDockerfile{
					Source: domain.DockerfileSourcePath{
						Path: s.GetBuild().GetDockerfile().GetDockerfilePath(),
					},
				}
			case v1.DockerfileSource_DOCKERFILE_SOURCE_INLINE:
				spec.BuildConfig = domain.BuildConfigDockerfile{
					Source: domain.DockerfileSourceInline{
						Content: s.GetBuild().GetDockerfile().GetDockerfileInline(),
					},
				}
			default:
				return domain.ApplicationSpec{}, errors.New("invalid dockerfile build config")
			}
		case v1.BuildStrategy_BUILD_STRATEGY_BUILDPACKS:
			spec.BuildConfig = domain.BuildConfigBuildpacks{
				Builder: s.GetBuild().GetBuildpacks().GetBuilder(),
			}
		default:
			return domain.ApplicationSpec{}, errors.New("invalid build strategy")
		}
	}
	if s.GetAccess() != nil {
		switch s.GetAccess().GetMode() {
		case v1.AccessMode_ACCESS_MODE_PUBLIC:
			spec.AccessMode = domain.AccessModePublic
		case v1.AccessMode_ACCESS_MODE_PRIVATE:
			spec.AccessMode = domain.AccessModePrivate
		case v1.AccessMode_ACCESS_MODE_MEMBERS_ONLY:
			spec.AccessMode = domain.AccessModeMembersOnly
		case v1.AccessMode_ACCESS_MODE_UNSPECIFIED:
		default:
			return domain.ApplicationSpec{}, errors.New("invalid access mode")
		}
	}
	if s.GetAccess().GetDomain() != nil {
		spec.Domain = &domain.Domain{
			Subdomain:  s.GetAccess().GetDomain().GetSubdomain(),
			RootDomain: s.GetAccess().GetDomain().GetRootDomain(),
		}
	}
	for key, val := range s.EnvironmentVariables {
		spec.Env = append(spec.Env, domain.KeyValue{Key: key, Value: val})
	}
	for key, val := range s.Secrets {
		spec.Secrets = append(spec.Secrets, domain.KeyValue{Key: key, Value: val})
	}
	return spec, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
