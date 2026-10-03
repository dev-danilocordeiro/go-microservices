package httpx

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
)

// NewApp cria o *fiber.App com o ErrorHandler de Problem Details e os
// middlewares básicos. Cada serviço só registra rotas nele.
func NewApp(service string, log *slog.Logger, problemTypeBase string) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      service,
		ErrorHandler: ErrorHandler(log, problemTypeBase),
	})
	app.Use(requestid.New())
	app.Use(recover.New()) // panic vira erro -> ErrorHandler -> 500 em Problem Details
	return app
}

// ErrInvalidBody é devolvido quando o JSON do corpo não pode ser decodificado.
var ErrInvalidBody = apperr.New(apperr.KindInvalid, "request.invalid_body", "corpo da requisição inválido")

// BindJSON decodifica o corpo em dst, traduzindo falhas para um *apperr.Error.
func BindJSON(c fiber.Ctx, dst any) error {
	if err := c.Bind().JSON(dst); err != nil {
		return ErrInvalidBody.Wrap(err)
	}
	return nil
}
