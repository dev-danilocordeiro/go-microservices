// Package memory implementa app.Repository em memória.
package memory

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/domain"
)

type Repository struct {
	mu     sync.RWMutex
	orders map[uuid.UUID]domain.Order
}

func New() *Repository {
	return &Repository{orders: make(map[uuid.UUID]domain.Order)}
}

func (r *Repository) Create(_ context.Context, o domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[o.ID] = o
	return nil
}

func (r *Repository) GetByID(_ context.Context, id uuid.UUID) (domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrNotFound
	}
	return o, nil
}
