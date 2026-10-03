package app

import (
	"github.com/dev-danilocordeiro/go-authkit"
	"github.com/dev-danilocordeiro/go-authkit/authz"
)

// Papéis (client roles do client "users-service" no Keycloak).
const (
	RoleAdmin     = "admin"
	RoleUsersRead = "users:read"
)

// Políticas do módulo. Ficam juntas, num arquivo só, para que "quem pode o
// quê" seja auditável lendo um único lugar.
var (
	// Ler um perfil (o recurso é o ID do usuário alvo):
	// a própria pessoa, um admin, ou um sistema com users:read.
	canRead = authz.Any(
		isSelf,
		authz.Role[string](RoleAdmin),
		authz.All(authz.Service[string](), authz.Role[string](RoleUsersRead)),
	)

	// Listar todos os perfis: só admin.
	canList = authz.Role[authz.None](RoleAdmin)

	// /users/me só faz sentido para pessoas.
	canAccessSelf = authz.User[authz.None]()
)

func isSelf(p authkit.Principal, userID string) bool {
	return p.IsUser() && p.Subject == userID
}
