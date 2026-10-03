package server_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/dev-danilocordeiro/go-authkit/authtest"
	"github.com/dev-danilocordeiro/go-authkit/s2s"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/httpx"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/server"
	"github.com/dev-danilocordeiro/go-microservices/services/users/usersapi"
	"github.com/dev-danilocordeiro/go-microservices/services/users/userstest"
)

// Teste de integração da cadeia inteira, sem Docker:
//
//	cliente ──► orders-service ──(s2s: exchange ou client credentials)──► users-service
//	                 │                                                       (real, em memória,
//	                 └──► authtest.TokenServer ("Keycloak")                   atrás de HTTP)

const grantExchange = "urn:ietf:params:oauth:grant-type:token-exchange"

type env struct {
	t      *testing.T
	orders *fiber.App
	users  *httptest.Server
	tokens *authtest.TokenServer
	iss    *authtest.Issuer // chave compartilhada; audiência base
	oIss   *authtest.Issuer // tokens para o orders-service
	uIss   *authtest.Issuer // tokens para o users-service
}

// newEnv sobe tudo. ordersRolesOnUsers são os papéis do service account do
// orders-service no users-service (em produção: users:read).
func newEnv(t *testing.T, ordersRolesOnUsers ...string) *env {
	t.Helper()
	iss := authtest.NewIssuer(t)
	e := &env{t: t, iss: iss, oIss: iss.WithAudience("orders-service"), uIss: iss.WithAudience("users-service")}

	e.users = userstest.Start(t, e.uIss.Verifier())
	e.tokens = authtest.NewTokenServer(t, iss, map[string]authtest.ServiceClient{
		"orders-service": {Secret: "orders-secret", Roles: map[string][]string{"users-service": ordersRolesOnUsers}},
	})
	tokens, err := s2s.New(s2s.Config{TokenURL: e.tokens.URL, ClientID: "orders-service", ClientSecret: "orders-secret"})
	if err != nil {
		t.Fatal(err)
	}
	// Mesma montagem do main.go.
	usersClient := usersapi.NewClient(e.users.URL, &http.Client{
		Timeout:   2 * time.Second,
		Transport: tokens.Transport(nil, "users-service", s2s.Auto),
	})
	e.orders = server.NewApp(server.Deps{
		Log:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier:        e.oIss.Verifier(),
		Users:           usersClient,
		ProblemTypeBase: "https://errors.example.com/",
	})
	return e
}

// adminToken: como no Keycloak, uma pessoa com papéis nos DOIS serviços.
func (e *env) adminToken() string {
	return e.iss.Token(map[string]any{
		"sub": "root", "azp": "app-web", "preferred_username": "root", "name": "Root", "email": "root@example.com",
		"aud": []string{"orders-service", "users-service"},
		"resource_access": map[string]any{
			"orders-service": map[string]any{"roles": []string{"admin", "orders:write"}},
			"users-service":  map[string]any{"roles": []string{"admin"}},
		},
	})
}

type response struct {
	Status int
	Body   []byte
}

func (r response) code(t *testing.T) string {
	t.Helper()
	var p httpx.Problem
	_ = json.Unmarshal(r.Body, &p)
	return p.Code
}

func (e *env) do(token, method, path, body string) response {
	e.t.Helper()
	req := httptest.NewRequestWithContext(e.t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", authtest.Bearer(token))
	}
	resp, err := e.orders.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return response{Status: resp.StatusCode, Body: b}
}

// usersList consulta o users-service diretamente, como admin.
func (e *env) usersList() []usersapi.User {
	e.t.Helper()
	req, _ := http.NewRequestWithContext(e.t.Context(), http.MethodGet, e.users.URL+"/v1/users", nil)
	req.Header.Set("Authorization", authtest.Bearer(e.uIss.UserToken("root", "admin")))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out []usersapi.User
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		e.t.Fatal(err)
	}
	return out
}

func TestPersonIdentityFlowsToUsersService(t *testing.T) {
	e := newEnv(t, "users:read")
	alice := e.oIss.UserToken("alice", "orders:write")

	for range 2 {
		if r := e.do(alice, "POST", "/v1/orders", `{"total_cents":1990}`); r.Status != 201 {
			t.Fatalf("alice cria pedido: %d %s", r.Status, r.Body)
		}
	}

	// O users-service viu A ALICE (token exchange) e criou o perfil dela no
	// primeiro acesso, pela política "a própria pessoa".
	if got := e.usersList(); len(got) != 1 || got[0].ID != "alice" {
		t.Fatalf("perfis no users-service: %+v", got)
	}
	// Dois pedidos, um exchange: o token trocado fica em cache.
	if n := e.tokens.Count(grantExchange); n != 1 {
		t.Fatalf("exchanges = %d, quer 1", n)
	}
}

func TestOrdersScenarios(t *testing.T) {
	e := newEnv(t, "users:read")
	var (
		alice   = e.oIss.UserToken("alice", "orders:write")
		bob     = e.oIss.UserToken("bob", "orders:write")
		billing = e.oIss.ServiceToken("billing", "orders:write", "orders:read")
	)
	created := e.do(alice, "POST", "/v1/orders", `{"total_cents":1990}`) // provisiona alice no users
	var order struct{ ID string }
	_ = json.Unmarshal(created.Body, &order)

	tests := []struct {
		name, token, method, path, body string
		status                          int
		code                            string
	}{
		{"anônimo", "", "POST", "/v1/orders", `{"total_cents":1}`, 401, "auth.unauthenticated"},
		{"para outra pessoa (barrado no orders)", alice, "POST", "/v1/orders", `{"user_id":"bob","total_cents":1}`, 403, "auth.forbidden"},
		{"validação", alice, "POST", "/v1/orders", `{"total_cents":0}`, 400, "orders.invalid"},
		{"dono lê", alice, "GET", "/v1/orders/" + order.ID, "", 200, ""},
		{"outro lê", bob, "GET", "/v1/orders/" + order.ID, "", 403, "auth.forbidden"},
		{"admin lê", e.adminToken(), "GET", "/v1/orders/" + order.ID, "", 200, ""},
		// sistema: orders chama users como ELE MESMO (client credentials)
		{"sistema cria para alice", billing, "POST", "/v1/orders", `{"user_id":"alice","total_cents":500}`, 201, ""},
		// users.not_found (404 do users-service, pela rede) vira orders.unknown_user
		{"sistema: usuário inexistente", billing, "POST", "/v1/orders", `{"user_id":"ninguem","total_cents":500}`, 422, "orders.unknown_user"},
		// admin via exchange: o token trocado mantém o papel admin no users-service
		{"admin cria para alice", e.adminToken(), "POST", "/v1/orders", `{"user_id":"alice","total_cents":500}`, 201, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := e.do(tt.token, tt.method, tt.path, tt.body)
			if r.Status != tt.status || r.code(t) != tt.code {
				t.Fatalf("got %d %q, quer %d %q (%s)", r.Status, r.code(t), tt.status, tt.code, r.Body)
			}
		})
	}
}

func TestUsersServiceDown(t *testing.T) {
	e := newEnv(t, "users:read")
	e.users.Close()

	r := e.do(e.oIss.UserToken("alice", "orders:write"), "POST", "/v1/orders", `{"total_cents":1}`)
	if r.Status != 503 || r.code(t) != "users.unavailable" {
		t.Fatalf("got %d %q, quer 503 users.unavailable (%s)", r.Status, r.code(t), r.Body)
	}
}

func TestOrdersServiceAccountWithoutUsersRead(t *testing.T) {
	e := newEnv(t) // service account do orders SEM users:read
	billing := e.oIss.ServiceToken("billing", "orders:write")

	// O users-service aplica a política dele à identidade do orders e nega.
	r := e.do(billing, "POST", "/v1/orders", `{"user_id":"alice","total_cents":1}`)
	if r.Status != 403 || r.code(t) != "auth.forbidden" {
		t.Fatalf("got %d %q (%s)", r.Status, r.code(t), r.Body)
	}
}
