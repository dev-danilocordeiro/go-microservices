// Package app contém os casos de uso de pedidos.
package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dev-danilocordeiro/go-authkit"

	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/domain"
	"github.com/dev-danilocordeiro/go-microservices/services/users/usersapi"
)

// Repository: mesmo contrato de erros do módulo users.
//   - GetByID: domain.ErrNotFound se não existe.
type Repository interface {
	Create(ctx context.Context, o domain.Order) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Order, error)
}

// Users é o que orders precisa do users-service. Em Go, a interface fica no
// CONSUMIDOR e pede só os métodos que ele usa. usersapi.Client (HTTP) a
// satisfaz sem saber que ela existe, e nos testes basta um fake de 1 método.
//
// Para este código não importa que a chamada atravessa a rede: os erros
// voltam como *apperr.Error com o mesmo Code do users-service.
type Users interface {
	GetUser(ctx context.Context, id string) (usersapi.User, error)
}

type Service struct {
	repo  Repository
	users Users
	now   func() time.Time
}

func NewService(repo Repository, u Users) *Service {
	return &Service{repo: repo, users: u, now: time.Now}
}

// Place cria um pedido. Para uma pessoa, userID vazio significa "para mim".
func (s *Service) Place(ctx context.Context, userID string, totalCents int64) (domain.Order, error) {
	userID = strings.TrimSpace(userID) // normaliza ANTES da política, para ela ver o mesmo valor que será gravado
	if p, ok := authkit.FromContext(ctx); ok && p.IsUser() && userID == "" {
		userID = p.Subject
	}
	if err := canPlaceFor.Check(ctx, userID); err != nil {
		return domain.Order{}, err
	}

	o, err := domain.NewOrder(userID, totalCents, s.now())
	if err != nil {
		return domain.Order{}, err
	}

	// O ctx carrega o principal e o token de entrada: o client HTTP (s2s)
	// decide com que identidade chamar o users-service. Pessoa -> token
	// exchange (o users vê a pessoa); sistema -> client credentials.
	if _, err := s.users.GetUser(ctx, userID); err != nil {
		// TRADUZIR quando o significado muda: "usuário não encontrado" no
		// contexto de criar pedido é uma regra de negócio violada (422).
		// Wrap mantém a causa: errors.Is(err, usersapi.ErrNotFound) continua true.
		if errors.Is(err, usersapi.ErrNotFound) {
			return domain.Order{}, domain.ErrUnknownUser.Wrap(err)
		}
		// PROPAGAR quando o significado não muda (403 do users, users fora do ar -> 503...).
		return domain.Order{}, err
	}

	if err := s.repo.Create(ctx, o); err != nil {
		return domain.Order{}, err
	}
	return o, nil
}

// Get lê um pedido. A política depende do pedido (quem é o dono), então
// primeiro carregamos e depois autorizamos.
func (s *Service) Get(ctx context.Context, rawID string) (domain.Order, error) {
	if _, ok := authkit.FromContext(ctx); !ok {
		// Sem isso, um anônimo descobriria se um ID existe (404 x 401).
		return domain.Order{}, authkit.ErrUnauthenticated
	}
	id, err := domain.ParseID(rawID)
	if err != nil {
		return domain.Order{}, err
	}
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	if err := canRead.Check(ctx, o); err != nil {
		return domain.Order{}, err
	}
	return o, nil
}
