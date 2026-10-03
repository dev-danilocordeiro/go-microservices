// Package server monta o orders-service (composition root). Separado do main
// para os testes subirem o serviço inteiro em memória.
package server

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dev-danilocordeiro/go-authkit"
	"github.com/dev-danilocordeiro/go-authkit/fiberauth"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/httpx"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/app"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/httpapi"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/memory"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/postgres"
)

type Deps struct {
	Log             *slog.Logger
	DB              *pgxpool.Pool    // nil => repositório em memória
	Verifier        authkit.Verifier // audiência: orders-service
	Users           app.Users        // em produção: usersapi.Client com transporte s2s
	ProblemTypeBase string
}

func NewApp(d Deps) *fiber.App {
	var repo app.Repository = memory.New()
	if d.DB != nil {
		repo = postgres.New(d.DB)
	}

	a := httpx.NewApp("orders-service", d.Log, d.ProblemTypeBase)
	a.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })

	v1 := a.Group("/v1", fiberauth.New(d.Verifier))
	httpapi.New(app.NewService(repo, d.Users)).Register(v1)
	return a
}
