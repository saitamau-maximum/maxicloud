package handler

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1/maxicloudv1connect"
	"github.com/saitamau-maximum/maxicloud/internal/usecase"
)

var _ maxicloudv1connect.UserServiceHandler = (*UserHandler)(nil)

type UserHandler struct {
	maxicloudv1connect.UnimplementedUserServiceHandler
	uc usecase.UserService
}

func NewUserHandler(uc usecase.UserService) *UserHandler {
	return &UserHandler{uc: uc}
}

func (h *UserHandler) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.GetUserResponse, error) {
	user, err := h.uc.GetUser(ctx, req.GetUserId())
	if err != nil {
		return nil, toConnectError(err)
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return &v1.GetUserResponse{User: toProtoUser(user)}, nil
}
