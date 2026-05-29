package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"golang.org/x/oauth2"
)

type CallbackResult struct {
	User       *domain.User
	Token      string
	RedirectTo string
}

type AuthService interface {
	Login(ctx context.Context, redirectTo string) (loginURL, state string, err error)
	Callback(ctx context.Context, code, state string) (*CallbackResult, error)
	GetCurrentUser(ctx context.Context, bearerToken string) (*domain.User, error)
}

type AuthConfig struct {
	Issuer           string
	ClientID         string
	RedirectURL      string
	StateSecret      string
	SessionSecret    string
	AllowedRedirects []string
}

type authService struct {
	cfg      AuthConfig
	userRepo domain.UserRepository
	oauth2   *oauth2.Config
}

func NewAuthService(cfg AuthConfig, userRepo domain.UserRepository) AuthService {
	oauthCfg := &oauth2.Config{
		ClientID:    cfg.ClientID,
		RedirectURL: cfg.RedirectURL,
		Scopes:      []string{"openid"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.Issuer + "/oauth/authorize",
			TokenURL: cfg.Issuer + "/oauth/access-token",
		},
	}
	return &authService{
		cfg:      cfg,
		userRepo: userRepo,
		oauth2:   oauthCfg,
	}
}

type stateClaims struct {
	jwt.RegisteredClaims
	RedirectTo   string `json:"redirect_to"`
	CodeVerifier string `json:"code_verifier"`
}

type sessionClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
}

func (s *authService) Login(ctx context.Context, redirectTo string) (string, string, error) {
	if err := s.validateRedirect(redirectTo); err != nil {
		return "", "", err
	}

	codeVerifier, codeChallenge, err := generatePKCE()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate PKCE: %w", err)
	}

	now := time.Now()
	claims := stateClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Audience:  jwt.ClaimStrings{"state"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		},
		RedirectTo:   redirectTo,
		CodeVerifier: codeVerifier,
	}
	stateToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.StateSecret))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign state token: %w", err)
	}

	authURL := s.oauth2.AuthCodeURL(stateToken,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	return authURL, stateToken, nil
}

func (s *authService) Callback(ctx context.Context, code, stateToken string) (*CallbackResult, error) {
	claims := &stateClaims{}
	_, err := jwt.ParseWithClaims(stateToken, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.StateSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid state token: %w", err)
	}
	if !claims.VerifyAudience("state", true) {
		return nil, fmt.Errorf("invalid state token audience")
	}

	if err := s.validateRedirect(claims.RedirectTo); err != nil {
		return nil, err
	}

	token, err := s.oauth2.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", claims.CodeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	userInfo, err := s.fetchUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch userinfo: %w", err)
	}

	user, err := s.userRepo.UpsertUser(ctx, *userInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert user: %w", err)
	}

	sessionToken, err := s.issueSessionToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &CallbackResult{
		User:       user,
		Token:      sessionToken,
		RedirectTo: claims.RedirectTo,
	}, nil
}

func (s *authService) GetCurrentUser(ctx context.Context, bearerToken string) (*domain.User, error) {
	token := strings.TrimPrefix(bearerToken, "Bearer ")
	if token == "" {
		return nil, nil
	}

	claims := &sessionClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.SessionSecret), nil
	})
	if err != nil {
		return nil, nil
	}
	if !claims.VerifyAudience("session", true) {
		return nil, nil
	}

	return s.userRepo.GetUserByID(ctx, claims.UserID)
}

func (s *authService) issueSessionToken(userID string) (string, error) {
	now := time.Now()
	claims := sessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "maxicloud",
			Audience:  jwt.ClaimStrings{"session"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
		UserID: userID,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.SessionSecret))
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

type userinfoResponse struct {
	Sub         string   `json:"sub"`
	DisplayID   string   `json:"display_id"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
}

func (s *authService) fetchUserInfo(ctx context.Context, accessToken string) (*domain.User, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken}))
	resp, err := client.Get(s.cfg.Issuer + "/oauth/resources/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info userinfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &domain.User{
		ID:          info.Sub,
		DisplayID:   info.DisplayID,
		DisplayName: info.DisplayName,
		Roles:       info.Roles,
	}, nil
}

func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}
