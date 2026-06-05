package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"golang.org/x/oauth2"
)

type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type Client struct {
	cfg      Config
	oauth2   *oauth2.Config
	mu       sync.Mutex
	verifier *coreoidc.IDTokenVerifier
}

func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		oauth2: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"openid", "profile", "read:roles"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  cfg.Issuer + "/oauth/authorize",
				TokenURL: cfg.Issuer + "/oauth/access-token",
			},
		},
	}
}

func (c *Client) AuthURL(state, nonce string) string {
	return c.oauth2.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce))
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := c.oauth2.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (c *Client) VerifyIDToken(ctx context.Context, rawIDToken, expectedNonce string) error {
	verifier, err := c.verifierFor(ctx)
	if err != nil {
		return err
	}
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return err
	}

	var claims struct {
		Nonce string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return err
	}
	if claims.Nonce == "" {
		return fmt.Errorf("nonce claim is missing")
	}
	if claims.Nonce != expectedNonce {
		return fmt.Errorf("nonce mismatch")
	}
	return nil
}

func (c *Client) FetchUserInfo(ctx context.Context, accessToken string) (*domain.OIDCUserInfo, error) {
	httpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken}))
	resp, err := httpClient.Get(c.cfg.Issuer + "/oauth/resources/userinfo")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var info struct {
		Sub               string   `json:"sub"`
		PreferredUsername string   `json:"preferred_username"`
		Nickname          string   `json:"nickname"`
		Roles             []string `json:"roles"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	roles := info.Roles
	if roles == nil {
		roles = []string{}
	}

	return &domain.OIDCUserInfo{
		ID:          info.Sub,
		DisplayID:   info.PreferredUsername,
		DisplayName: info.Nickname,
		Roles:       roles,
	}, nil
}

func (c *Client) verifierFor(ctx context.Context) (*coreoidc.IDTokenVerifier, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.verifier != nil {
		return c.verifier, nil
	}

	provider, err := coreoidc.NewProvider(ctx, c.cfg.Issuer)
	if err != nil {
		return nil, err
	}
	c.verifier = provider.Verifier(&coreoidc.Config{ClientID: c.cfg.ClientID})
	return c.verifier, nil
}

var _ domain.OIDCClient = (*Client)(nil)
