package handler

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1/maxicloudv1connect"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/service/deployment"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ maxicloudv1connect.DeploymentServiceHandler = (*DeploymentHandler)(nil)

type DeploymentHandler struct {
	maxicloudv1connect.UnimplementedDeploymentServiceHandler
	service deployment.DeploymentService
	history deployment.History
	watcher deployment.Watcher
}

func NewDeploymentHandler(
	deployService deployment.DeploymentService,
	history deployment.History,
	watcher deployment.Watcher,
) *DeploymentHandler {
	return &DeploymentHandler{
		service: deployService,
		history: history,
		watcher: watcher,
	}
}

func (h *DeploymentHandler) RetryDeployment(ctx context.Context, req *v1.RetryDeploymentRequest) (*v1.RetryDeploymentResponse, error) {
	deploy, err := h.service.Retry(ctx, req.GetDeploymentId())
	if err != nil {
		return nil, toConnectError(err)
	}
	if deploy == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return &v1.RetryDeploymentResponse{Deployment: toProtoDeployment(deploy)}, nil
}

func (h *DeploymentHandler) GetDeployment(ctx context.Context, req *v1.GetDeploymentRequest) (*v1.GetDeploymentResponse, error) {
	deploy, err := h.history.Get(ctx, req.GetDeploymentId())
	if err != nil {
		return nil, toConnectError(err)
	}
	if deploy == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return &v1.GetDeploymentResponse{Deployment: toProtoDeployment(deploy)}, nil
}

func (h *DeploymentHandler) ListDeployments(ctx context.Context, req *v1.ListDeploymentsRequest) (*v1.ListDeploymentsResponse, error) {
	deploys, err := h.history.List(ctx, req.GetApplicationId())
	if err != nil {
		return nil, toConnectError(err)
	}

	protoDeploys := make([]*v1.Deployment, 0, len(deploys))
	for i := range deploys {
		protoDeploys = append(protoDeploys, toProtoDeployment(&deploys[i]))
	}
	return &v1.ListDeploymentsResponse{Deployments: protoDeploys}, nil
}

func (h *DeploymentHandler) WatchDeployment(ctx context.Context, req *v1.WatchDeploymentRequest, stream *connect.ServerStream[v1.WatchDeploymentResponse]) error {
	events, err := h.watcher.WatchDeployment(ctx, req.GetDeploymentId())
	if err != nil {
		return toConnectError(err)
	}
	for event := range events {
		var protoEvent *v1.WatchDeploymentResponse
		switch e := event.(type) {
		case deployment.DeploymentStatusChangedEvent:
			var finishedAt *timestamppb.Timestamp
			if e.FinishedAt != nil {
				finishedAt = timestamppb.New(*e.FinishedAt)
			}
			protoEvent = &v1.WatchDeploymentResponse{
				Event: &v1.WatchDeploymentResponse_DeploymentStatusChanged{
					DeploymentStatusChanged: &v1.DeploymentStatusChangedEvent{
						Status:         toProtoDeploymentStatus(e.Status),
						ElapsedSeconds: e.ElapsedSeconds,
						FinishedAt:     finishedAt,
					},
				},
			}
		case deployment.DeploymentLogChunkEvent:
			protoEvent = &v1.WatchDeploymentResponse{
				Event: &v1.WatchDeploymentResponse_DeploymentLogChunk{
					DeploymentLogChunk: &v1.DeploymentLogChunkEvent{
						Line: e.Line,
					},
				},
			}
		default:
			continue
		}
		if err := stream.Send(protoEvent); err != nil {
			return toConnectError(err)
		}
	}
	return nil
}

func toProtoDeployment(d *domain.Deployment) *v1.Deployment {
	if d == nil {
		return nil
	}
	var finishedAt *timestamppb.Timestamp
	if d.FinishedAt != nil {
		finishedAt = timestamppb.New(*d.FinishedAt)
	}
	return &v1.Deployment{
		Id:            d.ID,
		ApplicationId: d.Spec.ApplicationID,
		OwnerUserId:   d.Spec.OwnerUserID,
		Commit: &v1.Commit{
			Sha:        d.Spec.Commit.SHA,
			Message:    d.Spec.Commit.Message,
			AuthorName: d.Spec.Commit.AuthorName,
			Timestamp:  timestamppb.New(d.Spec.Commit.Timestamp),
		},
		Status:     toProtoDeploymentStatus(d.Status),
		StartedAt:  timestamppb.New(d.StartedAt),
		FinishedAt: finishedAt,
	}
}

func toProtoDeploymentStatus(status domain.DeploymentStatus) v1.DeploymentStatus {
	switch status {
	case domain.DeploymentStatusQueued:
		return v1.DeploymentStatus_DEPLOYMENT_STATUS_IN_PROGRESS
	case domain.DeploymentStatusSucceeded:
		return v1.DeploymentStatus_DEPLOYMENT_STATUS_SUCCESS
	case domain.DeploymentStatusInProgress:
		return v1.DeploymentStatus_DEPLOYMENT_STATUS_IN_PROGRESS
	case domain.DeploymentStatusFailed:
		return v1.DeploymentStatus_DEPLOYMENT_STATUS_FAILED
	default:
		return v1.DeploymentStatus_DEPLOYMENT_STATUS_UNSPECIFIED
	}
}
