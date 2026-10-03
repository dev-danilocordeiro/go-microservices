package app

import (
	"github.com/dev-danilocordeiro/go-authkit"
	"github.com/dev-danilocordeiro/go-authkit/authz"

	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/domain"
)

// Papéis (client roles do client "orders-service" no Keycloak).
const (
	RoleAdmin       = "admin"
	RoleOrdersRead  = "orders:read"
	RoleOrdersWrite = "orders:write"
)

var (
	// Criar pedido para um usuário (o recurso é o ID do dono do pedido).
	// RBAC (precisa de orders:write) E ABAC (para si mesmo, ou admin, ou
	// sistema, que cria em nome de alguém).
	canPlaceFor = authz.All(
		authz.Role[string](RoleOrdersWrite),
		authz.Any(
			func(p authkit.Principal, ownerID string) bool { return p.IsUser() && p.Subject == ownerID },
			authz.Role[string](RoleAdmin),
			authz.Service[string](),
		),
	)

	// Ler um pedido: ABAC sobre o pedido carregado do banco.
	canRead = authz.Any(
		isOwner,
		authz.Role[domain.Order](RoleAdmin),
		authz.All(authz.Service[domain.Order](), authz.Role[domain.Order](RoleOrdersRead)),
	)
)

func isOwner(p authkit.Principal, o domain.Order) bool {
	return p.IsUser() && p.Subject == o.UserID
}
