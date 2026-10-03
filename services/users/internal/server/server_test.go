package server_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dev-danilocordeiro/go-authkit/authtest"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/httpx"
	"github.com/dev-danilocordeiro/go-microservices/services/users/internal/server"
)

func TestUsersPolicies(t *testing.T) {
	iss := authtest.NewIssuer(t).WithAudience("users-service")
	app := server.NewApp(server.Deps{
		Log:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier:        iss.Verifier(),
		ProblemTypeBase: "https://errors.example.com/",
	})
	do := func(token, path string) (status int, code, challenge string) {
		req := httptest.NewRequestWithContext(t.Context(), "GET", path, nil)
		if token != "" {
			req.Header.Set("Authorization", authtest.Bearer(token))
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		var p httpx.Problem
		_ = json.NewDecoder(resp.Body).Decode(&p)
		return resp.StatusCode, p.Code, resp.Header.Get("WWW-Authenticate")
	}

	var (
		alice     = iss.UserToken("alice")
		admin     = iss.UserToken("root", "admin")
		reader    = iss.ServiceToken("orders-service", "users:read")
		nonReader = iss.ServiceToken("other-service")
		// token de outro serviço: assinatura válida, audiência errada
		wrongAud = iss.WithAudience("orders-service").UserToken("alice")
	)
	if s, _, _ := do(alice, "/v1/users/me"); s != 200 {
		t.Fatalf("/me: %d", s)
	}

	tests := []struct {
		name, token, path string
		status            int
		code              string
	}{
		{"anônimo", "", "/v1/users/me", 401, "auth.unauthenticated"},
		{"token de outra audiência", wrongAud, "/v1/users/me", 401, "auth.invalid_token"},
		{"a própria pessoa", alice, "/v1/users/alice", 200, ""},
		{"outra pessoa", iss.UserToken("bob"), "/v1/users/alice", 403, "auth.forbidden"},
		{"admin", admin, "/v1/users/alice", 200, ""},
		{"sistema com users:read", reader, "/v1/users/alice", 200, ""},
		{"sistema sem users:read", nonReader, "/v1/users/alice", 403, "auth.forbidden"},
		{"sistema: inexistente", reader, "/v1/users/ninguem", 404, "users.not_found"},
		{"lista sem admin", alice, "/v1/users", 403, "auth.forbidden"},
		{"admin lista", admin, "/v1/users", 200, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, challenge := do(tt.token, tt.path)
			if status != tt.status || code != tt.code {
				t.Fatalf("got %d %q, quer %d %q", status, code, tt.status, tt.code)
			}
			if status == 401 && !strings.HasPrefix(challenge, "Bearer") {
				t.Fatalf("401 sem WWW-Authenticate: %q", challenge)
			}
		})
	}
}
