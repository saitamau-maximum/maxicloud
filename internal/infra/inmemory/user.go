package inmemory

import (
	"context"
	"sync"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type userRepository struct {
	mu   sync.RWMutex
	data map[string]*domain.User
}

func NewUserRepository() domain.UserRepository {
	return &userRepository{data: make(map[string]*domain.User)}
}

func (r *userRepository) GetUserByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.data[id]
	if !ok {
		return nil, nil
	}
	cp := *u
	return &cp, nil
}

func (r *userRepository) UpsertUser(_ context.Context, user domain.User) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[user.ID] = &user
	cp := user
	return &cp, nil
}
