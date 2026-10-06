package auth

import "context"

type contextKey int

const (
	userIDContextKey contextKey = iota
	rolesContextKey
)

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

func UserID(ctx context.Context) string {
	userID, _ := ctx.Value(userIDContextKey).(string)
	return userID
}

func WithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, rolesContextKey, roles)
}

func Roles(ctx context.Context) []string {
	roles, _ := ctx.Value(rolesContextKey).([]string)
	return roles
}
