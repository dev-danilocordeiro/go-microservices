// Command users é o users-service: perfis de usuário (a identidade é do Keycloak).
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dev-danilocordeiro/go-authkit/oidcauth"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/config"
	"github.com/dev-danilocordeiro/go-microservices/internal/platform/database"
	"github.com/dev-danilocordeiro/go-microservices/internal/platform/runner"
	"github.com/dev-danilocordeiro/go-microservices/services/users/internal/server"
	"github.com/dev-danilocordeiro/go-microservices/services/users/migrations"
)

func main() { runner.Main("users-service", run) }

func run(ctx context.Context, log *slog.Logger) error {
	cfg, err := config.Load("users-service")
	if err != nil {
		return err
	}

	var db *pgxpool.Pool
	if cfg.DBDriver == "postgres" {
		if db, err = database.Open(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
		defer db.Close()
		if err := database.Migrate(ctx, db, migrations.FS); err != nil {
			return err
		}
	}

	verifier, err := oidcauth.New(ctx, oidcauth.Config{
		IssuerURL:    cfg.OIDC.IssuerURL,
		DiscoveryURL: cfg.OIDC.DiscoveryURL,
		Audience:     cfg.OIDC.Audience,
	})
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	app := server.NewApp(server.Deps{Log: log, DB: db, Verifier: verifier, ProblemTypeBase: cfg.ProblemTypeBase})
	return runner.Serve(ctx, log, app, cfg.HTTPAddr)
}
