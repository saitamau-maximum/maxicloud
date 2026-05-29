package domain

import (
	"context"
	"time"
)

type User struct {
	ID          string
	DisplayID   string
	DisplayName string
	Roles       []string
	CreatedAt   time.Time
}

type UserRepository interface {
	GetUserByID(ctx context.Context, id string) (*User, error)
	UpsertUser(ctx context.Context, user User) (*User, error)
}

type contextKey int

const userContextKey contextKey = iota

func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey).(*User)
	return u
}
