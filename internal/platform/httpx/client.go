package httpx

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/dev-danilocordeiro/go-microservices/internal/platform/apperr"
)

// Este arquivo é o caminho de VOLTA: transforma a resposta de erro de outro
// serviço (Problem Details) de novo em *apperr.Error. Assim um serviço que
// chama outro pela rede recebe o mesmo erro, com o mesmo Code, que receberia
// numa chamada local, e errors.Is(err, usersapi.ErrNotFound) funciona.

// Unavailable é o erro de "a dependência não respondeu": rede, timeout, 5xx.
// Não é culpa de quem chamou nem bug nosso, por isso 503 e não 500.
func Unavailable(service string, cause error) *apperr.Error {
	return apperr.New(apperr.KindUnavailable, service+".unavailable",
		"serviço "+service+" indisponível").Wrap(cause)
}

// ErrorFromResponse converte uma resposta não-2xx de outro serviço.
//
//   - 4xx com Problem Details -> mesmo Kind/Code/mensagem/campos do serviço remoto
//   - 401                     -> Internal: o token que NÓS enviamos foi recusado
//     (configuração s2s errada); o cliente original não tem culpa
//   - 5xx ou corpo inesperado -> Unavailable (503)
func ErrorFromResponse(resp *http.Response, service string) error {
	cause := fmt.Errorf("%s respondeu %d %s", service, resp.StatusCode, resp.Request.URL.Path)

	var p Problem
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	isProblem := ct == problemContentType && json.Unmarshal(body, &p) == nil && p.Code != ""

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return apperr.Internal(fmt.Errorf("%w: credenciais serviço-a-serviço recusadas (%s)", cause, p.Code))
	case resp.StatusCode >= 500 || !isProblem:
		return Unavailable(service, cause)
	}

	kind, ok := kindOf(resp.StatusCode)
	if !ok {
		return Unavailable(service, cause)
	}
	e := apperr.New(kind, p.Code, p.Detail)
	for _, f := range p.Errors {
		e = e.WithField(f.Field, f.Message)
	}
	return e.Wrap(cause)
}

// kindOf é o inverso de StatusOf, só para os status que fazem sentido
// propagar como estão.
func kindOf(status int) (apperr.Kind, bool) {
	switch status {
	case http.StatusBadRequest:
		return apperr.KindInvalid, true
	case http.StatusForbidden:
		return apperr.KindForbidden, true
	case http.StatusNotFound:
		return apperr.KindNotFound, true
	case http.StatusConflict:
		return apperr.KindConflict, true
	case http.StatusUnprocessableEntity:
		return apperr.KindUnprocessable, true
	default:
		return 0, false
	}
}
