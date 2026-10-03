// Package app contém os casos de uso de usuários. Orquestra domínio,
// autorização e persistência, mas depende só de interfaces ("portas").
//
// A autorização é feita AQUI, não no handler HTTP: o handler fica fino e a
// regra fica junto do caso de uso, testável sem HTTP.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/dev-danilocordeiro/go-authkit"
	"github.com/dev-danilocordeiro/go-authkit/authz"

	"github.com/dev-danilocordeiro/go-microservices/services/users/internal/domain"
)

// Repository é a porta de persistência. Postgres e memória a implementam;
// trocar de banco = escrever outra implementação, sem mexer aqui.
//
// Contrato de erros (toda implementação DEVE respeitar):
//   - GetByID: domain.ErrNotFound se não existe.
//   - Falhas de infraestrutura: qualquer outro erro (vira 500 na borda).
type Repository interface {
	Save(ctx context.Context, u domain.User) error // insere ou atualiza
	GetByID(ctx context.Context, id string) (domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
}

type Service struct {
	repo Repository
	now  func() time.Time // injetável para testes determinísticos
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// Me devolve o perfil de quem está chamando, criando-o no primeiro acesso
// (provisionamento "just-in-time") e sincronizando nome/e-mail com o token.
func (s *Service) Me(ctx context.Context) (domain.User, error) {
	if err := canAccessSelf.Check(ctx, authz.None{}); err != nil {
		return domain.User{}, err
	}
	p, _ := authkit.FromContext(ctx) // Check garantiu que existe
	return s.provision(ctx, p)
}

// Get lê o perfil de um usuário, aplicando a política canRead.
func (s *Service) Get(ctx context.Context, id string) (domain.User, error) {
	if err := canRead.Check(ctx, id); err != nil {
		return domain.User{}, err
	}
	u, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		// A própria pessoa ainda não tem perfil: cria agora.
		if p, _ := authkit.FromContext(ctx); isSelf(p, id) {
			return s.provision(ctx, p)
		}
	}
	return u, err
}

func (s *Service) List(ctx context.Context) ([]domain.User, error) {
	if err := canList.Check(ctx, authz.None{}); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *Service) provision(ctx context.Context, p authkit.Principal) (domain.User, error) {
	u, err := s.repo.GetByID(ctx, p.Subject)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		if u, err = domain.NewUser(p.Subject, p.Name, p.Email, s.now()); err != nil {
			return domain.User{}, err
		}
	case err != nil:
		return domain.User{}, err
	case !u.SyncIdentity(p.Name, p.Email, s.now()):
		return u, nil // nada mudou, nada a salvar
	}
	if err := s.repo.Save(ctx, u); err != nil {
		return domain.User{}, err
	}
	return u, nil
}
