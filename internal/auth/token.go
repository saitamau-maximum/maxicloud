package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type stateClaims struct {
	jwt.RegisteredClaims
	RedirectTo string `json:"redirect_to"`
	Nonce      string `json:"nonce"`
}

type sessionClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
}

type ParsedStateToken struct {
	RedirectTo string
	Nonce      string
}

func GenerateNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func IssueStateToken(redirectTo, nonce, sessionSecret string, now time.Time) (string, error) {
	claims := stateClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Audience:  jwt.ClaimStrings{"state"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		},
		RedirectTo: redirectTo,
		Nonce:      nonce,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(sessionSecret))
}

func ParseStateToken(stateToken, sessionSecret string) (*ParsedStateToken, error) {
	if stateToken == "" {
		return nil, fmt.Errorf("state is required")
	}

	claims := &stateClaims{}
	_, err := jwt.ParseWithClaims(
		stateToken,
		claims,
		func(t *jwt.Token) (any, error) {
			return []byte(sessionSecret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid state token: %w", err)
	}
	if !claims.VerifyAudience("state", true) {
		return nil, fmt.Errorf("invalid state token audience")
	}

	return &ParsedStateToken{
		RedirectTo: claims.RedirectTo,
		Nonce:      claims.Nonce,
	}, nil
}

func IssueSessionToken(userID, sessionSecret string, now time.Time) (string, error) {
	claims := sessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "maxicloud",
			Audience:  jwt.ClaimStrings{"session"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
		UserID: userID,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(sessionSecret))
}

func ParseSessionToken(token, sessionSecret string) (string, bool) {
	if token == "" {
		return "", false
	}

	claims := &sessionClaims{}
	_, err := jwt.ParseWithClaims(
		token,
		claims,
		func(t *jwt.Token) (any, error) {
			return []byte(sessionSecret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !claims.VerifyAudience("session", true) || claims.UserID == "" {
		return "", false
	}
	return claims.UserID, true
}
