package httpx_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
	"github.com/dev-danilocordeiro/go-microservices/internal/platform/httpx"
)

var errRemoteNotFound = apperr.New(apperr.KindNotFound, "users.not_found", "x")

const problemCT = "application/problem+json; charset=utf-8"

// remoteError simula a resposta de erro de outro serviço e a converte.
func remoteError(t *testing.T, status int, contentType, body string) error {
	t.Helper()
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", contentType)
	rec.WriteHeader(status)
	_, _ = rec.WriteString(body)
	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()
	resp.Request = httptest.NewRequestWithContext(t.Context(), "GET", "/v1/users/x", nil)
	return httpx.ErrorFromResponse(resp, "users")
}

func TestErrorFromResponse(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
		kind        apperr.Kind
		code        string
		fields      int
	}{
		{"404 preserva o código remoto", 404, problemCT, `{"status":404,"code":"users.not_found","detail":"não achei"}`, apperr.KindNotFound, "users.not_found", 0},
		{"400 preserva os campos", 400, problemCT, `{"status":400,"code":"users.invalid","errors":[{"field":"id","message":"x"}]}`, apperr.KindInvalid, "users.invalid", 1},
		{"403 propaga", 403, problemCT, `{"status":403,"code":"auth.forbidden"}`, apperr.KindForbidden, "auth.forbidden", 0},
		{"401 é erro NOSSO (credencial s2s)", 401, problemCT, `{"status":401,"code":"auth.invalid_token"}`, apperr.KindInternal, "internal", 0},
		{"500 remoto vira indisponível", 500, problemCT, `{"status":500,"code":"internal"}`, apperr.KindUnavailable, "users.unavailable", 0},
		{"resposta sem Problem (proxy)", 502, "text/html", "<html>bad gateway</html>", apperr.KindUnavailable, "users.unavailable", 0},
		{"429 vira indisponível", 429, problemCT, `{"status":429,"code":"rate_limited"}`, apperr.KindUnavailable, "users.unavailable", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ae := apperr.As(remoteError(t, tt.status, tt.contentType, tt.body))
			if ae.Kind != tt.kind || ae.Code != tt.code || len(ae.Fields) != tt.fields {
				t.Fatalf("got kind=%v code=%q fields=%v", ae.Kind, ae.Code, ae.Fields)
			}
		})
	}

	t.Run("errors.Is funciona do outro lado da rede", func(t *testing.T) {
		if err := remoteError(t, 404, problemCT, `{"status":404,"code":"users.not_found"}`); !errors.Is(err, errRemoteNotFound) {
			t.Fatal("deveria casar pelo Code")
		}
	})
}
