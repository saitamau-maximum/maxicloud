package handler

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type stubUserService struct {
	user *domain.User
	err  error
}

func (s *stubUserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.user, s.err
}

func TestUserHandlerGetUser(t *testing.T) {
	h := NewUserHandler(&stubUserService{
		user: &domain.User{
			ID:          "user-1",
			DisplayID:   "kouta",
			DisplayName: "Kouta",
			Roles:       []string{"member"},
		},
	})

	res, err := h.GetUser(context.Background(), &v1.GetUserRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("GetUser returned error: %v", err)
	}
	if res.GetUser().GetId() != "user-1" {
		t.Fatalf("expected id user-1, got %q", res.GetUser().GetId())
	}
	if res.GetUser().GetDisplayId() != "kouta" {
		t.Fatalf("expected display_id kouta, got %q", res.GetUser().GetDisplayId())
	}
	if res.GetUser().GetDisplayName() != "Kouta" {
		t.Fatalf("expected display_name Kouta, got %q", res.GetUser().GetDisplayName())
	}
	if len(res.GetUser().GetRoles()) != 1 || res.GetUser().GetRoles()[0] != "member" {
		t.Fatalf("expected roles [member], got %v", res.GetUser().GetRoles())
	}
}

func TestUserHandlerGetUserNotFound(t *testing.T) {
	h := NewUserHandler(&stubUserService{user: nil})

	_, err := h.GetUser(context.Background(), &v1.GetUserRequest{UserId: "missing"})
	if err == nil {
		t.Fatalf("expected error")
	}
	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected *connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Fatalf("expected code %v, got %v", connect.CodeNotFound, connectErr.Code())
	}
}
