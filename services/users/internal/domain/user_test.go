package domain_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
	"github.com/dev-danilocordeiro/go-microservices/services/users/internal/domain"
)

// O domínio (e os testes dele) não pode depender do contrato nem de HTTP.
func TestDomainHasNoHTTPDeps(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "-test", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, bad := range []string{"/usersapi", "/platform/httpx", "gofiber/fiber"} {
		if strings.Contains(string(out), bad) {
			t.Errorf("domain depende de %q", bad)
		}
	}
}

// Os Codes são o contrato público: mudar aqui quebra quem chama pela rede.
func TestErrorCodes(t *testing.T) {
	_, err := domain.NewUser(" ", "a", "b", time.Time{})
	if !errors.Is(err, domain.ErrInvalid) || apperr.As(err).Code != "users.invalid" {
		t.Fatalf("NewUser sem id: %v", err)
	}
	if ae := apperr.As(domain.ErrNotFound); ae.Kind != apperr.KindNotFound || ae.Code != "users.not_found" {
		t.Fatalf("ErrNotFound = %+v", ae)
	}
}
