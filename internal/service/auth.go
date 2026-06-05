package service

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/saitamau-maximum/maxicloud/internal/auth"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type CallbackResult struct {
	User       *domain.User
	Token      string
	RedirectTo string
}

type AuthService interface {
	Login(ctx context.Context, redirectTo string) (loginURL, state string, err error)
	Callback(ctx context.Context, code, state string) (*CallbackResult, error)
}

type AuthConfig struct {
	SessionSecret    string
	AllowedRedirects []string
}

type authService struct {
	cfg        AuthConfig
	userRepo   domain.UserRepository
	oidcClient domain.OIDCClient
}

func NewAuthService(cfg AuthConfig, userRepo domain.UserRepository, oidcClient domain.OIDCClient) AuthService {
	return &authService{
		cfg:        cfg,
		userRepo:   userRepo,
		oidcClient: oidcClient,
	}
}

func (s *authService) Login(ctx context.Context, redirectTo string) (string, string, error) {
	if err := s.validateRedirect(redirectTo); err != nil {
		return "", "", err
	}

	nonce, err := auth.GenerateNonce()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	stateToken, err := auth.IssueStateToken(redirectTo, nonce, s.cfg.SessionSecret, time.Now())
	if err != nil {
		return "", "", fmt.Errorf("failed to sign state token: %w", err)
	}

	return s.oidcClient.AuthURL(stateToken, nonce), stateToken, nil
}

func (s *authService) Callback(ctx context.Context, code, stateToken string) (*CallbackResult, error) {
	if code == "" {
		return nil, fmt.Errorf("authorization code is required")
	}

	state, err := auth.ParseStateToken(stateToken, s.cfg.SessionSecret)
	if err != nil {
		return nil, err
	}
	if err := s.validateRedirect(state.RedirectTo); err != nil {
		return nil, err
	}

	token, err := s.oidcClient.ExchangeCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	rawIDToken, _ := token.Extra("id_token").(string)
	if rawIDToken == "" {
		return nil, fmt.Errorf("id_token not found in token response")
	}
	if err := s.oidcClient.VerifyIDToken(ctx, rawIDToken, state.Nonce); err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}

	userInfo, err := s.oidcClient.FetchUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch userinfo: %w", err)
	}

	user, err := s.userRepo.Upsert(ctx, domain.User{
		ID:          userInfo.ID,
		DisplayID:   userInfo.DisplayID,
		DisplayName: userInfo.DisplayName,
		Roles:       userInfo.Roles,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert user: %w", err)
	}

	sessionToken, err := auth.IssueSessionToken(user.ID, s.cfg.SessionSecret, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to issue session token: %w", err)
	}

	return &CallbackResult{
		User:       user,
		Token:      sessionToken,
		RedirectTo: state.RedirectTo,
	}, nil
}

func (s *authService) validateRedirect(redirectTo string) error {
	if len(s.cfg.AllowedRedirects) == 0 {
		return nil
	}
	u, err := url.Parse(redirectTo)
	if err != nil {
		return fmt.Errorf("invalid redirect_to: %w", err)
	}
	for _, allowed := range s.cfg.AllowedRedirects {
		a, err := url.Parse(allowed)
		if err != nil {
			continue
		}
		if u.Scheme == a.Scheme && u.Host == a.Host {
			return nil
		}
	}
	return fmt.Errorf("redirect_to %q is not allowed", redirectTo)
}
