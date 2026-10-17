// Package usersapi é o CONTRATO PÚBLICO do users-service: o JSON das
// respostas, os códigos de erro e um client HTTP pronto.
//
// É o único pacote de services/users que outro serviço pode importar (o
// resto está em internal/ e o compilador barra). Mudanças aqui são mudanças
// de API: mantenha compatibilidade (só adicione campos, nunca renomeie Codes).
package usersapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
	"github.com/dev-danilocordeiro/go-microservices/internal/platform/httpx"
)

// User é o perfil como trafega no JSON.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Erros do contrato. apperr.Error compara por Code, então o erro reconstruído
// a partir do Problem Details da resposta é errors.Is-igual a estes. O domínio
// (internal/domain) define erros com os mesmos Codes; não troque um sem o outro.
var (
	ErrNotFound = apperr.New(apperr.KindNotFound, "users.not_found", "usuário não encontrado")
	ErrInvalid  = apperr.New(apperr.KindInvalid, "users.invalid", "dados de usuário inválidos")
)

const service = "users"

// Client chama o users-service. O *http.Client recebido decide COMO
// autenticar (ex.: Transport do go-authkit/s2s); o Client só cuida do
// protocolo: rotas, JSON e tradução de erros.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, hc *http.Client) *Client {
	return &Client{baseURL: strings.TrimSuffix(baseURL, "/"), http: hc}
}

// GetUser devolve o perfil. Erros possíveis (todos *apperr.Error):
// ErrNotFound, ErrInvalid, auth.forbidden, users.unavailable.
func (c *Client) GetUser(ctx context.Context, id string) (User, error) {
	var u User
	err := c.get(ctx, "/v1/users/"+url.PathEscape(id), &u)
	return u, err
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return apperr.Internal(err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// Rede, timeout, ou falha ao obter o token de saída.
		return httpx.Unavailable(service, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return httpx.ErrorFromResponse(resp, service)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return httpx.Unavailable(service, fmt.Errorf("resposta inválida: %w", err))
	}
	return nil
}
