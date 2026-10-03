// Package httpapi expõe os casos de uso de pedidos via REST (Fiber).
package httpapi

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/httpx"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/app"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/domain"
)

type Handler struct {
	svc *app.Service
}

func New(svc *app.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(r fiber.Router) {
	g := r.Group("/orders")
	g.Post("/", h.create)
	g.Get("/:id", h.get)
}

type createOrderRequest struct {
	UserID     string `json:"user_id"` // opcional para pessoas: vazio = "para mim"
	TotalCents int64  `json:"total_cents"`
}

type orderResponse struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	TotalCents int64     `json:"total_cents"`
	CreatedAt  time.Time `json:"created_at"`
}

func toResponse(o domain.Order) orderResponse {
	return orderResponse{ID: o.ID.String(), UserID: o.UserID, TotalCents: o.TotalCents, CreatedAt: o.CreatedAt}
}

func (h *Handler) create(c fiber.Ctx) error {
	var req createOrderRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}
	o, err := h.svc.Place(c.Context(), req.UserID, req.TotalCents)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(toResponse(o))
}

func (h *Handler) get(c fiber.Ctx) error {
	o, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(toResponse(o))
}
