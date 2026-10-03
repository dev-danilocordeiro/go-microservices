// Package postgres implementa app.Repository com pgx.
package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/domain"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, o domain.Order) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO orders (id, user_id, total_cents, created_at) VALUES ($1, $2, $3, $4)`,
		o.ID, o.UserID, o.TotalCents, o.CreatedAt)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (domain.Order, error) {
	var o domain.Order
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, total_cents, created_at FROM orders WHERE id = $1`, id).
		Scan(&o.ID, &o.UserID, &o.TotalCents, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Order{}, apperr.Internal(err)
	}
	return o, nil
}
