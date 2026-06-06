package handler

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1/maxicloudv1connect"
	"github.com/saitamau-maximum/maxicloud/internal/auth"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/service"
)

var _ maxicloudv1connect.UserServiceHandler = (*UserHandler)(nil)

type UserHandler struct {
	maxicloudv1connect.UnimplementedUserServiceHandler
	service service.UserService
}

func NewUserHandler(svc service.UserService) *UserHandler {
	return &UserHandler{service: svc}
}

func (h *UserHandler) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.GetUserResponse, error) {
	user, err := h.service.Get(ctx, req.GetUserId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return &v1.GetUserResponse{User: toProtoUser(user)}, nil
}

func (h *UserHandler) GetMe(ctx context.Context, req *v1.GetMeRequest) (*v1.GetMeResponse, error) {
	userID := auth.UserID(ctx)
	if userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	user, err := h.service.Get(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return &v1.GetMeResponse{User: toProtoUser(user)}, nil
}

func toProtoUser(u *domain.User) *v1.User {
	if u == nil {
		return nil
	}
	return &v1.User{
		Id:          u.ID,
		DisplayId:   u.DisplayID,
		DisplayName: u.DisplayName,
		Roles:       u.Roles,
	}
}
