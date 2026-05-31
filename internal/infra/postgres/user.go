package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
	"github.com/saitamau-maximum/maxicloud/internal/infra/postgres/db"
)

type userRepository struct {
	q *db.Queries
}

var _ domain.UserRepository = (*userRepository)(nil)

func NewUserRepository(pool *pgxpool.Pool) domain.UserRepository {
	return &userRepository{q: db.New(pool)}
}

func (r *userRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return userRowToDomain(row), nil
}

func (r *userRepository) UpsertUser(ctx context.Context, u domain.User) (*domain.User, error) {
	row, err := r.q.UpsertUser(ctx, db.UpsertUserParams{
		ID:          u.ID,
		DisplayID:   u.DisplayID,
		DisplayName: u.DisplayName,
		Roles:       u.Roles,
	})
	if err != nil {
		return nil, err
	}
	return userRowToDomain(row), nil
}

func userRowToDomain(row db.User) *domain.User {
	return &domain.User{
		ID:          row.ID,
		DisplayID:   row.DisplayID,
		DisplayName: row.DisplayName,
		Roles:       row.Roles,
		CreatedAt:   row.CreatedAt.Time,
	}
}
