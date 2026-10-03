// Package apperr define o erro de aplicação usado por todos os módulos.
//
// A ideia central: o erro carrega SIGNIFICADO (Kind + Code), não um status HTTP.
// Quem chama via Go (outro módulo) recebe o mesmo *Error que o handler REST
// recebe; só a borda HTTP traduz Kind -> status e monta o Problem Details.
package apperr

import (
	"errors"
	"fmt"
)

// Kind é a categoria do erro. É o que a borda (HTTP, gRPC, CLI...) usa para
// decidir como representar o erro. Domínio nunca fala em "404" ou "500".
type Kind uint8

const (
	KindInternal      Kind = iota // falha inesperada; detalhes nunca vazam para o cliente
	KindInvalid                   // entrada inválida (validação)
	KindNotFound                  // recurso não existe
	KindConflict                  // viola uma regra de unicidade/estado
	KindUnauthorized              // não autenticado
	KindForbidden                 // autenticado, mas sem permissão
	KindUnprocessable             // entrada bem formada, mas a regra de negócio recusa
	KindUnavailable               // uma dependência (outro serviço) está fora ou não respondeu
)

func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "invalid"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindUnauthorized:
		return "unauthorized"
	case KindForbidden:
		return "forbidden"
	case KindUnprocessable:
		return "unprocessable"
	case KindUnavailable:
		return "unavailable"
	default:
		return "internal"
	}
}

// FieldError descreve um problema em um campo específico da entrada.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error é o erro de aplicação.
//
// Code é um identificador estável e legível por máquina ("users.not_found").
// Clientes (outros módulos, front-end) devem comparar Code, nunca Message.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Fields  []FieldError
	cause   error
}

// New cria um erro. Normalmente usado para declarar erros de domínio como
// variáveis de pacote, que funcionam como "sentinelas":
//
//	var ErrNotFound = apperr.New(apperr.KindNotFound, "users.not_found", "usuário não encontrado")
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return e.Code + ": " + e.Message
}

// Unwrap permite que errors.Is/errors.As enxerguem a causa original.
func (e *Error) Unwrap() error { return e.cause }

// Is faz errors.Is(err, ErrNotFound) funcionar mesmo depois de With*/Wrap,
// que devolvem cópias. Dois *Error são "o mesmo erro" se têm o mesmo Code.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// Os métodos abaixo devolvem CÓPIAS. Nunca altere a sentinela, pois ela é
// compartilhada por todas as goroutines.

// WithMessage troca a mensagem legível por humanos, mantendo Kind e Code.
func (e *Error) WithMessage(format string, args ...any) *Error {
	c := e.clone()
	c.Message = fmt.Sprintf(format, args...)
	return c
}

// WithField adiciona um erro de campo (útil para validação).
func (e *Error) WithField(field, message string) *Error {
	c := e.clone()
	c.Fields = append(c.Fields, FieldError{Field: field, Message: message})
	return c
}

// Wrap anexa a causa técnica (ex.: erro do driver do banco). A causa vai
// para os logs, mas nunca para a resposta HTTP.
func (e *Error) Wrap(cause error) *Error {
	c := e.clone()
	c.cause = cause
	return c
}

func (e *Error) clone() *Error {
	c := *e
	c.Fields = append([]FieldError(nil), e.Fields...)
	return &c
}

// HasFields informa se há erros de campo acumulados.
func (e *Error) HasFields() bool { return len(e.Fields) > 0 }

// Internal embrulha uma falha inesperada (banco fora do ar, bug...) num *Error
// de KindInternal, preservando a causa para log.
func Internal(cause error) *Error {
	return &Error{Kind: KindInternal, Code: "internal", Message: "erro interno", cause: cause}
}

// Interfaces comportamentais: erros de OUTROS pacotes (ex.: go-authkit) se
// classificam implementando estes métodos, sem que apperr precise importá-los.
// É o mesmo idioma de net.Error.Timeout() na biblioteca padrão.
type (
	unauthenticatedError interface{ Unauthenticated() bool }
	forbiddenError       interface{ Forbidden() bool }
	codedError           interface {
		Code() string
		Message() string
	}
)

// As extrai o *Error de qualquer cadeia de erros. Erros que se classificam
// via interfaces comportamentais são convertidos; o resto (um panic
// recuperado, um erro cru do driver) vira Internal. Assim a borda sempre tem
// um *Error para traduzir.
func As(err error) *Error {
	if err == nil {
		return nil
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}

	var (
		unauth unauthenticatedError
		forbid forbiddenError
	)
	switch {
	case errors.As(err, &unauth) && unauth.Unauthenticated():
		return fromCoded(err, KindUnauthorized, "auth.unauthenticated", "autenticação necessária")
	case errors.As(err, &forbid) && forbid.Forbidden():
		return fromCoded(err, KindForbidden, "auth.forbidden", "acesso negado")
	}
	return Internal(err)
}

func fromCoded(err error, kind Kind, code, message string) *Error {
	var ce codedError
	if errors.As(err, &ce) {
		code, message = ce.Code(), ce.Message()
	}
	return New(kind, code, message).Wrap(err)
}

// KindOf é um atalho para As(err).Kind.
func KindOf(err error) Kind { return As(err).Kind }
