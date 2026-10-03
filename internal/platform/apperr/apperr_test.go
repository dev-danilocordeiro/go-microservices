package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
)

var errThing = apperr.New(apperr.KindNotFound, "thing.not_found", "coisa não encontrada")

func TestIsSurvivesCopiesAndWrapping(t *testing.T) {
	derived := errThing.WithMessage("coisa %d não encontrada", 42).WithField("id", "x")
	wrapped := fmt.Errorf("camada de cima: %w", derived)

	if !errors.Is(wrapped, errThing) {
		t.Fatal("errors.Is deveria reconhecer o erro pelo Code mesmo após cópia e %w")
	}
	if got := apperr.KindOf(wrapped); got != apperr.KindNotFound {
		t.Fatalf("KindOf = %v, quer not_found", got)
	}
}

func TestCopiesDoNotMutateSentinel(t *testing.T) {
	_ = errThing.WithField("a", "b")
	if errThing.HasFields() {
		t.Fatal("WithField alterou a sentinela compartilhada")
	}
}

func TestWrapKeepsCauseChain(t *testing.T) {
	cause := errors.New("driver: connection refused")
	outer := apperr.New(apperr.KindUnprocessable, "outer", "x").Wrap(errThing.Wrap(cause))

	for _, target := range []error{errThing, cause} {
		if !errors.Is(outer, target) {
			t.Fatalf("cadeia perdeu %v", target)
		}
	}
}

func TestAsTurnsUnknownErrorsIntoInternal(t *testing.T) {
	ae := apperr.As(errors.New("boom"))
	if ae.Kind != apperr.KindInternal || ae.Code != "internal" {
		t.Fatalf("erro desconhecido deveria virar internal, veio %+v", ae)
	}
}
