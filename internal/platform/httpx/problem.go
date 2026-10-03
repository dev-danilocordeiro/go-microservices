// Package httpx contém o que é comum à borda HTTP: tradução de erros para
// Problem Details (RFC 9457) e o servidor Fiber.
package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
)

const problemContentType = "application/problem+json"

// Problem é o corpo de erro definido pela RFC 9457 ("Problem Details for
// HTTP APIs"). Code e Errors são "extension members", permitidos pela RFC.
type Problem struct {
	Type     string              `json:"type"`
	Title    string              `json:"title"`
	Status   int                 `json:"status"`
	Detail   string              `json:"detail,omitempty"`
	Instance string              `json:"instance,omitempty"`
	Code     string              `json:"code,omitempty"`
	Errors   []apperr.FieldError `json:"errors,omitempty"`
}

// StatusOf é o ÚNICO lugar do sistema que mapeia significado -> HTTP status.
func StatusOf(k apperr.Kind) int {
	switch k {
	case apperr.KindInvalid:
		return http.StatusBadRequest
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindUnprocessable:
		return http.StatusUnprocessableEntity
	case apperr.KindUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// ErrorHandler é instalado no fiber.Config. Todo handler apenas faz
// `return err`; este handler decide como o erro vira resposta.
//
// typeBase é o prefixo da URI de "type" (ex.: "https://api.exemplo.com/problems/").
// Vazio => "about:blank", como recomenda a RFC quando não há documentação.
func ErrorHandler(log *slog.Logger, typeBase string) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		p := toProblem(err, typeBase)
		p.Instance = c.Path()

		if p.Status >= 500 {
			// A causa real vai para o log, com contexto; o cliente recebe só o genérico.
			log.ErrorContext(c.Context(), "request failed",
				"method", c.Method(), "path", c.Path(), "status", p.Status, "error", err)
		}

		if p.Status == http.StatusUnauthorized {
			// RFC 6750: um 401 de API com Bearer token deve dizer como se autenticar.
			challenge := `Bearer realm="api"`
			if p.Code == "auth.invalid_token" {
				challenge += `, error="invalid_token"`
			}
			c.Set(fiber.HeaderWWWAuthenticate, challenge)
		}

		return c.Status(p.Status).JSON(p, problemContentType)
	}
}

func toProblem(err error, typeBase string) Problem {
	// Erros do próprio Fiber (rota inexistente, método não permitido, body
	// malformado...) chegam como *fiber.Error.
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return Problem{
			Type:   "about:blank",
			Title:  http.StatusText(fe.Code),
			Status: fe.Code,
			Detail: fe.Message,
		}
	}

	ae := apperr.As(err)
	status := StatusOf(ae.Kind)
	p := Problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Code:   ae.Code,
		Errors: ae.Fields,
	}
	if ae.Kind != apperr.KindInternal {
		p.Detail = ae.Message // nunca expomos a mensagem de erros internos
		if typeBase != "" {
			p.Type = typeBase + ae.Code
		}
	}
	return p
}
