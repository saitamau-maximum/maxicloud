package handler

import (
	"net/http"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/saitamau-maximum/maxicloud/internal/usecase"
)

type AuthHandler struct {
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
	log := ctrl.Log.WithName("auth")
	if authErr := r.URL.Query().Get("error"); authErr != "" {
		desc := r.URL.Query().Get("error_description")
		log.Error(nil, "oidc callback returned error", "error", authErr, "description", desc)
		http.Error(w, authErr+": "+desc, http.StatusUnauthorized)
		return
	}

	result, err := h.uc.Callback(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"))
	if err != nil {
		log.Error(err, "callback failed")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if result == nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, result.RedirectTo+"?token="+result.Token, http.StatusFound)
}
