package handler

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1/maxicloudv1connect"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProjectHandler struct {
	maxicloudv1connect.UnimplementedProjectServiceHandler
	uc usecase.ProjectService
}

func NewProjectHandler(uc usecase.ProjectService) *ProjectHandler {
	return &ProjectHandler{uc: uc}
}

func (h *ProjectHandler) CreateProject(ctx context.Context, req *v1.CreateProjectRequest) (*v1.CreateProjectResponse, error) {
	project, err := h.uc.CreateProject(ctx, req.Name, req.Description, req.OwnerUserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &v1.CreateProjectResponse{Project: toProtoProject(project)}, nil
}

func (h *ProjectHandler) GetProject(ctx context.Context, req *v1.GetProjectRequest) (*v1.GetProjectResponse, error) {
	project, err := h.uc.GetProject(ctx, req.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return &v1.GetProjectResponse{Project: toProtoProject(project)}, nil
}

func (h *ProjectHandler) ListProjects(ctx context.Context, req *v1.ListProjectsRequest) (*v1.ListProjectsResponse, error) {
	projects, err := h.uc.ListProjects(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var protoProjects []*v1.Project
	for _, p := range projects {
		protoProjects = append(protoProjects, toProtoProject(p))
	}
	return &v1.ListProjectsResponse{Projects: protoProjects}, nil
}

func (h *ProjectHandler) UpdateProject(ctx context.Context, req *v1.UpdateProjectRequest) (*v1.UpdateProjectResponse, error) {
	project, err := h.uc.UpdateProject(ctx, usecase.UpdateProjectParams{
		ID:          req.ProjectId,
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     req.OwnerUserId,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &v1.UpdateProjectResponse{Project: toProtoProject(project)}, nil
}

func (h *ProjectHandler) DeleteProject(ctx context.Context, req *v1.DeleteProjectRequest) (*v1.DeleteProjectResponse, error) {
	if err := h.uc.DeleteProject(ctx, req.ProjectId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &v1.DeleteProjectResponse{}, nil
}

func toProtoProject(p *domain.Project) *v1.Project {
	return &v1.Project{
		Id:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		OwnerUserId: p.OwnerID,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}
}
