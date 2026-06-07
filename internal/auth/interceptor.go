package auth

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
)

var errUnauthenticated = errors.New("authentication required")

func NewInterceptor(sessionSecret string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			userID, ok := ParseSessionToken(sessionToken(req), sessionSecret)
			if !ok {
				return nil, connect.NewError(connect.CodeUnauthenticated, errUnauthenticated)
			}
			ctx = WithUserID(ctx, userID)
			return next(ctx, req)
		}
	}
}

func sessionToken(req connect.AnyRequest) string {
	httpReq := &http.Request{Header: req.Header()}
	token, _ := ReadSessionCookie(httpReq)
	return token
}
