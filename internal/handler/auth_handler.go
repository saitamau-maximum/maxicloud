package handler

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	v1 "github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1"
	"github.com/saitamau-maximum/maxicloud/gen/maxicloud/v1/maxicloudv1connect"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/usecase"
)

var _ maxicloudv1connect.AuthServiceHandler = (*AuthHandler)(nil)

type AuthHandler struct {
	maxicloudv1connect.UnimplementedAuthServiceHandler
	uc usecase.AuthService
}

func NewAuthHandler(uc usecase.AuthService) *AuthHandler {
	return &AuthHandler{uc: uc}
}

// GET /auth/login?redirect_to=...
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	redirectTo := r.URL.Query().Get("redirect_to")
	loginURL, _, err := h.uc.Login(r.Context(), redirectTo)
	if err != nil {
		http.Error(w, "login unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, loginURL, http.StatusFound)
}

// GET /auth/callback?code=xxx&state=xxx
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	result, err := h.uc.Callback(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"))
	if err != nil || result == nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, result.RedirectTo+"?token="+result.Token, http.StatusFound)
}

func (h *AuthHandler) Me(ctx context.Context, req *v1.MeRequest) (*v1.MeResponse, error) {
	user := domain.UserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	return &v1.MeResponse{
		User: toProtoUser(user),
	}, nil
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
