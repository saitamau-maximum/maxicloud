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
	Get(ctx context.Context, id string) (*User, error)
	Upsert(ctx context.Context, user User) (*User, error)
}
