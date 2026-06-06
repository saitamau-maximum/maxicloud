package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	SessionCookieName  = "maxicloud_session"
	StateCookieName    = "maxicloud_oidc_state"
	NonceCookieName    = "maxicloud_oidc_nonce"
	RedirectCookieName = "maxicloud_oidc_redirect_to"
)

type LoginFlowCookies struct {
	State      string
	Nonce      string
	RedirectTo string
}

func SetLoginFlowCookies(w http.ResponseWriter, r *http.Request, cookies LoginFlowCookies) {
	setCookie(w, r, http.Cookie{
		Name:     StateCookieName,
		Value:    cookies.State,
		Path:     "/auth/callback",
		MaxAge:   int(10 * time.Minute.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	setCookie(w, r, http.Cookie{
		Name:     NonceCookieName,
		Value:    cookies.Nonce,
		Path:     "/auth/callback",
		MaxAge:   int(10 * time.Minute.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	setCookie(w, r, http.Cookie{
		Name:     RedirectCookieName,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(cookies.RedirectTo)),
		Path:     "/auth/callback",
		MaxAge:   int(10 * time.Minute.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ReadLoginFlowCookies(r *http.Request) (*LoginFlowCookies, error) {
	state, err := readCookie(r, StateCookieName)
	if err != nil {
		return nil, err
	}
	nonce, err := readCookie(r, NonceCookieName)
	if err != nil {
		return nil, err
	}
	redirectTo, err := readCookie(r, RedirectCookieName)
	if err != nil {
		return nil, err
	}
	redirectBytes, err := base64.RawURLEncoding.DecodeString(redirectTo)
	if err != nil {
		return nil, fmt.Errorf("invalid %s cookie", RedirectCookieName)
	}
	return &LoginFlowCookies{
		State:      state,
		Nonce:      nonce,
		RedirectTo: string(redirectBytes),
	}, nil
}

func ClearLoginFlowCookies(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, r, StateCookieName, "/auth/callback")
	clearCookie(w, r, NonceCookieName, "/auth/callback")
	clearCookie(w, r, RedirectCookieName, "/auth/callback")
}

func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	setCookie(w, r, http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(24 * time.Hour.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ReadSessionCookie(r *http.Request) (string, bool) {
	value, err := readCookie(r, SessionCookieName)
	if err != nil {
		return "", false
	}
	return value, true
}

func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, r, SessionCookieName, "/")
}

func setCookie(w http.ResponseWriter, r *http.Request, cookie http.Cookie) {
	cookie.Secure = isSecureRequest(r)
	http.SetCookie(w, &cookie)
}

func clearCookie(w http.ResponseWriter, r *http.Request, name, path string) {
	setCookie(w, r, http.Cookie{
		Name:     name,
		Path:     path,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func readCookie(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", fmt.Errorf("%s cookie is required", name)
	}
	if cookie.Value == "" {
		return "", fmt.Errorf("%s cookie is empty", name)
	}
	return cookie.Value, nil
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
