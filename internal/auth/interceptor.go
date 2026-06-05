package auth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
)

var errUnauthenticated = errors.New("authentication required")

func NewAuthInterceptor(sessionSecret string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			userID, ok := ParseSessionToken(bearerToken(req.Header().Get("Authorization")), sessionSecret)
			if !ok {
				return nil, connect.NewError(connect.CodeUnauthenticated, errUnauthenticated)
			}
			ctx = WithUserID(ctx, userID)
			return next(ctx, req)
		}
	}
}

func bearerToken(value string) string {
	return strings.TrimPrefix(value, "Bearer ")
}
