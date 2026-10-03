// Package memory implementa app.Repository em memória. Serve para testes e
// para rodar a aplicação sem banco (DB_DRIVER=memory).
package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/dev-danilocordeiro/go-microservices/services/users/internal/domain"
)

type Repository struct {
	mu    sync.RWMutex
	users map[string]domain.User
}

func New() *Repository {
	return &Repository{users: make(map[string]domain.User)}
}

func (r *Repository) Save(_ context.Context, u domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[u.ID] = u
	return nil
}

func (r *Repository) GetByID(_ context.Context, id string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (r *Repository) List(_ context.Context) ([]domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, u)
	}
	slices.SortFunc(out, func(a, b domain.User) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, nil
}
