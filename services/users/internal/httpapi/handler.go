// Package httpapi expõe os casos de uso de usuários via REST (Fiber).
//
// Repare que nenhum handler escolhe status de erro nem checa permissão: eles
// só fazem `return err`. A autorização está no Service e o
// httpx.ErrorHandler monta o Problem Details.
package httpapi

import (
	"github.com/gofiber/fiber/v3"

	"github.com/dev-danilocordeiro/go-microservices/services/users/internal/app"
	"github.com/dev-danilocordeiro/go-microservices/services/users/internal/domain"
	"github.com/dev-danilocordeiro/go-microservices/services/users/usersapi"
)

type Handler struct {
	svc *app.Service
}

func New(svc *app.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(r fiber.Router) {
	g := r.Group("/users")
	g.Get("/me", h.me) // antes de /:id, senão "me" seria lido como um ID
	g.Get("/", h.list)
	g.Get("/:id", h.get)
}

// A resposta usa o DTO do contrato público (usersapi.User): o JSON que este
// handler escreve é exatamente o que o usersapi.Client lê.
func toResponse(u domain.User) usersapi.User {
	return usersapi.User{ID: u.ID, Name: u.Name, Email: u.Email, CreatedAt: u.CreatedAt}
}

func (h *Handler) me(c fiber.Ctx) error {
	u, err := h.svc.Me(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(toResponse(u))
}

func (h *Handler) get(c fiber.Ctx) error {
	u, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(toResponse(u))
}

func (h *Handler) list(c fiber.Ctx) error {
	users, err := h.svc.List(c.Context())
	if err != nil {
		return err
	}
	out := make([]usersapi.User, len(users))
	for i, u := range users {
		out[i] = toResponse(u)
	}
	return c.JSON(out)
}
