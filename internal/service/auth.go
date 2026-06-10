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

type LoginResult struct {
	LoginURL   string
	State      string
	Nonce      string
	RedirectTo string
}

type AuthService interface {
	Login(ctx context.Context, redirectTo string) (*LoginResult, error)
	Callback(ctx context.Context, code, redirectTo, nonce string) (*CallbackResult, error)
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

func (s *authService) Login(ctx context.Context, redirectTo string) (*LoginResult, error) {
	if err := s.validateRedirect(redirectTo); err != nil {
		return nil, err
	}

	state, err := auth.GenerateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	nonce, err := auth.GenerateNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	return &LoginResult{
		LoginURL:   s.oidcClient.AuthURL(state, nonce),
		State:      state,
		Nonce:      nonce,
		RedirectTo: redirectTo,
	}, nil
}

func (s *authService) Callback(ctx context.Context, code, redirectTo, nonce string) (*CallbackResult, error) {
	if code == "" {
		return nil, fmt.Errorf("authorization code is required")
	}
	if nonce == "" {
		return nil, fmt.Errorf("nonce is required")
	}
	if err := s.validateRedirect(redirectTo); err != nil {
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
	if err := s.oidcClient.VerifyIDToken(ctx, rawIDToken, nonce); err != nil {
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

	sessionToken, err := auth.IssueSessionToken(user.ID, user.Roles, s.cfg.SessionSecret, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to issue session token: %w", err)
	}

	return &CallbackResult{
		User:       user,
		Token:      sessionToken,
		RedirectTo: redirectTo,
	}, nil
}

func (s *authService) validateRedirect(redirectTo string) error {
	if redirectTo == "" {
		return fmt.Errorf("redirect_to is required")
	}
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
