// Package userstest sobe um users-service REAL, com repositório em memória,
// num servidor HTTP de teste. É o que outros serviços usam nos testes de
// integração: chamam o users-service de verdade pela rede, com as políticas
// e os erros de verdade, sem Docker nem banco.
//
// Só pode ser importado em arquivos _test.go (regra depguard no .golangci.yml).
package userstest

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3/middleware/adaptor"

	"github.com/dev-danilocordeiro/go-authkit"

	"github.com/dev-danilocordeiro/go-microservices/services/users/internal/server"
)

// Start sobe o serviço; ele é derrubado no fim do teste. verifier deve
// aceitar tokens com audiência "users-service".
func Start(t testing.TB, verifier authkit.Verifier) *httptest.Server {
	t.Helper()
	app := server.NewApp(server.Deps{
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier: verifier,
	})
	srv := httptest.NewServer(adaptor.FiberApp(app))
	t.Cleanup(srv.Close)
	return srv
}
