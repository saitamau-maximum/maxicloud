package auth

import (
	"context"
	"strings"

	"connectrpc.com/connect"
)

func NewOptionalAuthInterceptor(sessionSecret string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			userID, ok := ParseSessionToken(bearerToken(req.Header().Get("Authorization")), sessionSecret)
			if ok {
				ctx = WithUserID(ctx, userID)
			}
			return next(ctx, req)
		}
	}
}

func NewAuthInterceptor(sessionSecret string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			userID, ok := ParseSessionToken(bearerToken(req.Header().Get("Authorization")), sessionSecret)
			if !ok {
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}
			ctx = WithUserID(ctx, userID)
			return next(ctx, req)
		}
	}
}

func bearerToken(value string) string {
	return strings.TrimPrefix(value, "Bearer ")
}
