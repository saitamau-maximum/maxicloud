package usecase

import (
	"context"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type UserService interface {
	GetUser(ctx context.Context, id string) (*domain.User, error)
}

type userService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.GetUserByID(ctx, id)
}
