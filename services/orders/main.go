// Command orders é o orders-service. Chama o users-service para validar o
// dono do pedido, autenticando com o go-authkit/s2s.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dev-danilocordeiro/go-authkit/oidcauth"
	"github.com/dev-danilocordeiro/go-authkit/s2s"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/config"
	"github.com/dev-danilocordeiro/go-microservices/internal/platform/database"
	"github.com/dev-danilocordeiro/go-microservices/internal/platform/runner"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/server"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/migrations"
	"github.com/dev-danilocordeiro/go-microservices/services/users/usersapi"
)

func main() { runner.Main("orders-service", run) }

func run(ctx context.Context, log *slog.Logger) error {
	cfg, err := config.Load("orders-service")
	if err != nil {
		return err
	}
	env, err := config.Require("USERS_URL", "OIDC_CLIENT_SECRET")
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

	// Identidade do orders-service para chamar outros serviços.
	tokens, err := s2s.New(s2s.Config{
		TokenURL:     cfg.OIDC.TokenURL(),
		ClientID:     cfg.OIDC.ClientID,
		ClientSecret: cfg.OIDC.ClientSecret,
	})
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	users := usersapi.NewClient(env["USERS_URL"], &http.Client{
		// Sempre um timeout: sem ele, um users-service travado trava o orders junto.
		Timeout:   5 * time.Second,
		Transport: tokens.Transport(nil, "users-service", s2s.Auto),
	})

	app := server.NewApp(server.Deps{
		Log:             log,
		DB:              db,
		Verifier:        verifier,
		Users:           users,
		ProblemTypeBase: cfg.ProblemTypeBase,
	})
	return runner.Serve(ctx, log, app, cfg.HTTPAddr)
}
