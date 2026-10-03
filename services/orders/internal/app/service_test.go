package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dev-danilocordeiro/go-authkit"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/app"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/domain"
	"github.com/dev-danilocordeiro/go-microservices/services/orders/internal/memory"
	"github.com/dev-danilocordeiro/go-microservices/services/users/usersapi"
)

// fakeUsers implementa app.Users: como a interface tem 1 método, o fake é trivial.
type fakeUsers struct{ err error }

func (f fakeUsers) GetUser(_ context.Context, id string) (usersapi.User, error) {
	if f.err != nil {
		return usersapi.User{}, f.err
	}
	return usersapi.User{ID: id}, nil
}

func TestPlace(t *testing.T) {
	infraErr := apperr.Internal(errors.New("connection refused"))
	alice := authkit.Principal{Subject: "alice", Kind: authkit.KindUser, Roles: []string{app.RoleOrdersWrite}}

	tests := []struct {
		name      string
		principal *authkit.Principal
		users     fakeUsers
		userID    string
		total     int64
		wantErr   error
		wantKind  apperr.Kind
		alsoMatch error // erro original que deve continuar na cadeia
	}{
		{name: "ok, para si mesma", principal: &alice, total: 1000},
		{name: "sem principal", total: 1000, wantErr: authkit.ErrUnauthenticated, wantKind: apperr.KindUnauthorized},
		{name: "para outra pessoa", principal: &alice, userID: "bob", total: 1000, wantErr: authkit.ErrForbidden, wantKind: apperr.KindForbidden},
		{name: "validação", principal: &alice, total: 0, wantErr: domain.ErrInvalid, wantKind: apperr.KindInvalid},
		{
			name: "usuário inexistente é traduzido para 422", principal: &alice, total: 1000,
			users:   fakeUsers{err: usersapi.ErrNotFound},
			wantErr: domain.ErrUnknownUser, wantKind: apperr.KindUnprocessable, alsoMatch: usersapi.ErrNotFound,
		},
		{
			name: "falha de infra é propagada", principal: &alice, total: 1000,
			users:   fakeUsers{err: infraErr},
			wantErr: infraErr, wantKind: apperr.KindInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if tt.principal != nil {
				ctx = authkit.WithPrincipal(ctx, *tt.principal)
			}
			svc := app.NewService(memory.New(), tt.users)
			_, err := svc.Place(ctx, tt.userID, tt.total)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("erro inesperado: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, quer %v", err, tt.wantErr)
			}
			if got := apperr.KindOf(err); got != tt.wantKind {
				t.Fatalf("kind = %v, quer %v", got, tt.wantKind)
			}
			if tt.alsoMatch != nil && !errors.Is(err, tt.alsoMatch) {
				t.Fatalf("causa original %v perdida na cadeia", tt.alsoMatch)
			}
		})
	}
}
