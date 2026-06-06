package handler

import (
	"net/http"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/saitamau-maximum/maxicloud/internal/auth"
	"github.com/saitamau-maximum/maxicloud/internal/service"
)

type AuthHandler struct {
	service service.AuthService
}

func NewAuthHandler(svc service.AuthService) *AuthHandler {
	return &AuthHandler{service: svc}
}

// GET /auth/login?redirect_to=...
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	redirectTo := r.URL.Query().Get("redirect_to")
	result, err := h.service.Login(r.Context(), redirectTo)
	if err != nil {
		http.Error(w, "login unavailable", http.StatusInternalServerError)
		return
	}
	auth.SetLoginFlowCookies(w, r, auth.LoginFlowCookies{
		State:      result.State,
		Nonce:      result.Nonce,
		RedirectTo: result.RedirectTo,
	})
	http.Redirect(w, r, result.LoginURL, http.StatusFound)
}

// GET /auth/callback?code=xxx&state=xxx
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	log := ctrl.Log.WithName("auth")
	defer auth.ClearLoginFlowCookies(w, r)

	if authErr := r.URL.Query().Get("error"); authErr != "" {
		desc := r.URL.Query().Get("error_description")
		log.Error(nil, "oidc callback returned error", "error", authErr, "description", desc)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	cookies, err := auth.ReadLoginFlowCookies(r)
	if err != nil {
		log.Error(err, "oidc callback cookies are missing")
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	if state := r.URL.Query().Get("state"); state == "" || state != cookies.State {
		log.Error(nil, "oidc callback state mismatch")
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	result, err := h.service.Callback(
		r.Context(),
		r.URL.Query().Get("code"),
		cookies.RedirectTo,
		cookies.Nonce,
	)
	if err != nil {
		log.Error(err, "callback failed")
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	if result == nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	auth.SetSessionCookie(w, r, result.Token)
	http.Redirect(w, r, result.RedirectTo, http.StatusFound)
}
