// Package database abre a conexão com o Postgres e roda as migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Open cria o pool de conexões e garante que o banco está acessível.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	return pool, nil
}

// Migrate aplica as migrations do serviço. Cada serviço embute as suas e
// tem o seu próprio banco (database-per-service): nenhum serviço lê a
// tabela de outro; dados de outro serviço só pela API dele.
func Migrate(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func(db *sql.DB) { _ = db.Close() }(db)

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations)
	if err != nil {
		return fmt.Errorf("database: migrate: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("database: migrate: %w", err)
	}
	return nil
}
