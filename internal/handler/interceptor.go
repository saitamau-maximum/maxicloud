package handler

import (
	"context"

	"connectrpc.com/connect"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/usecase"
)

func NewOptionalAuthInterceptor(authSvc usecase.AuthService) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			token := req.Header().Get("Authorization")
			user, _ := authSvc.Me(ctx, token)
			if user != nil {
				ctx = domain.WithUser(ctx, user)
			}
			return next(ctx, req)
		}
	}
}

func NewAuthInterceptor(authSvc usecase.AuthService) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			token := req.Header().Get("Authorization")
			user, err := authSvc.Me(ctx, token)
			if err != nil || user == nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}
			ctx = domain.WithUser(ctx, user)
			return next(ctx, req)
		}
	}
}
