package domain

import (
	"context"
	"time"
)

type Project struct {
	ID          string
	Name        string
	Description string
	OwnerID     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UpdateProjectParams struct {
	ID          string
	Name        *string
	Description *string
	OwnerID     *string
	UpdatedAt   time.Time
}

type ProjectRepository interface {
	Create(ctx context.Context, project Project) (string, error)
	Get(ctx context.Context, id string) (*Project, error)
	List(ctx context.Context) ([]*Project, error)
	Update(ctx context.Context, params UpdateProjectParams) error
	Delete(ctx context.Context, id string) error
	// Preview用のProjectを作成
	CreatePreview(ctx context.Context, original Application, prNumber int) error
}
