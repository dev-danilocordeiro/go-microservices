// Package domain contém as regras de negócio de pedidos.
package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
)

var (
	ErrNotFound = apperr.New(apperr.KindNotFound, "orders.not_found", "pedido não encontrado")
	ErrInvalid  = apperr.New(apperr.KindInvalid, "orders.invalid", "dados de pedido inválidos")
	// ErrUnknownUser: a requisição é válida, mas o usuário referenciado não
	// existe. Para quem cria o pedido isso é 422, não 404: o recurso
	// /orders existe; quem não existe é algo que ele aponta.
	ErrUnknownUser = apperr.New(apperr.KindUnprocessable, "orders.unknown_user", "usuário do pedido não existe")
)

type Order struct {
	ID         uuid.UUID
	UserID     string // "sub" do dono no Keycloak
	TotalCents int64  // dinheiro em centavos (inteiro), nunca float
	CreatedAt  time.Time
}

func NewOrder(rawUserID string, totalCents int64, now time.Time) (Order, error) {
	verr := ErrInvalid
	userID := strings.TrimSpace(rawUserID)
	if userID == "" {
		verr = verr.WithField("user_id", "obrigatório")
	}
	if totalCents <= 0 {
		verr = verr.WithField("total_cents", "deve ser maior que zero")
	}
	if verr.HasFields() {
		return Order{}, verr
	}
	return Order{ID: uuid.New(), UserID: userID, TotalCents: totalCents, CreatedAt: now.UTC()}, nil
}

func ParseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, ErrInvalid.WithField("id", "deve ser um UUID").Wrap(err)
	}
	return id, nil
}
