package auth

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type sessionClaims struct {
	jwt.RegisteredClaims
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
}

func GenerateState() (string, error) { return generateRandomToken() }

func GenerateNonce() (string, error) { return generateRandomToken() }

func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func IssueSessionToken(userID string, roles []string, sessionSecret string, now time.Time) (string, error) {
	claims := sessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "maxicloud",
			Audience:  jwt.ClaimStrings{"session"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
		UserID: userID,
		Roles:  roles,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(sessionSecret))
}

func ParseSessionToken(token, sessionSecret string) (userID string, roles []string, ok bool) {
	if token == "" {
		return "", nil, false
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
		return "", nil, false
	}
	return claims.UserID, claims.Roles, true
}
