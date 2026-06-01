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
