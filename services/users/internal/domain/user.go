// Package domain contém as regras de negócio de usuários. Não conhece HTTP,
// SQL nem Fiber, só Go puro.
//
// Identidade (senha, MFA, recuperação de conta) é do Keycloak. Este módulo
// guarda só o PERFIL local do usuário, identificado pelo "sub" do token.
package domain

import (
	"strings"
	"time"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
)

// Erros do domínio. Os Codes são os mesmos do contrato público (usersapi) e,
// como apperr.Error compara por Code, errors.Is casa os dois lados. O domínio
// não importa usersapi: quem traduz para o contrato é a camada HTTP.
var (
	ErrNotFound = apperr.New(apperr.KindNotFound, "users.not_found", "usuário não encontrado")
	ErrInvalid  = apperr.New(apperr.KindInvalid, "users.invalid", "dados de usuário inválidos")
)

type User struct {
	ID        string // = "sub" do Keycloak; não assumimos formato (pode vir de federação)
	Name      string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewUser cria o perfil a partir dos dados de identidade.
func NewUser(id, name, email string, now time.Time) (User, error) {
	if strings.TrimSpace(id) == "" {
		return User{}, ErrInvalid.WithField("id", "obrigatório")
	}
	now = now.UTC()
	return User{
		ID:        id,
		Name:      strings.TrimSpace(name),
		Email:     strings.ToLower(strings.TrimSpace(email)),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// SyncIdentity atualiza os dados que vêm do provedor de identidade. Devolve
// true se algo mudou (para o chamador saber se precisa persistir).
func (u *User) SyncIdentity(name, email string, now time.Time) bool {
	name, email = strings.TrimSpace(name), strings.ToLower(strings.TrimSpace(email))
	if u.Name == name && u.Email == email {
		return false
	}
	u.Name, u.Email, u.UpdatedAt = name, email, now.UTC()
	return true
}
