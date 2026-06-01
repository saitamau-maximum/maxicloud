package domain

import (
	"context"

	"golang.org/x/oauth2"
)

type OIDCUserInfo struct {
	ID          string
	DisplayID   string
	DisplayName string
	Roles       []string
}

type OIDCClient interface {
	AuthURL(state, nonce string) string
	ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)
	VerifyIDToken(ctx context.Context, rawIDToken, expectedNonce string) error
	FetchUserInfo(ctx context.Context, accessToken string) (*OIDCUserInfo, error)
}
