# go-microservices

Template de **microsserviços** em Go com Fiber v3, PostgreSQL (pgx, um banco
por serviço), autenticação/autorização com **Keycloak** e chamadas entre
serviços autenticadas, via
[go-authkit](https://github.com/dev-danilocordeiro/go-authkit). Erros em
**Problem Details (RFC 9457)** que atravessam a rede sem perder o significado.

É o irmão do [go-modular-monolith](https://github.com/dev-danilocordeiro/go-modular-monolith):
os mesmos módulos (`users` e `orders`), as mesmas políticas e o mesmo modelo
de erro, só que separados por HTTP. Comparar os dois mostra exatamente o que
muda (e o que não muda) ao passar de módulo para serviço.

## Começando

```bash
go install golang.org/x/tools/cmd/gonew@latest
gonew github.com/dev-danilocordeiro/go-microservices github.com/voce/minha-plataforma
```

```bash
make test        # não precisa de nada rodando
make up          # Postgres + Keycloak
make run-users   # :8091
make run-orders  # :8092 (em outro terminal)
# ou tudo em containers:
make docker-up

TOKEN=$(make -s token U=alice)       # alice | bob | admin (senha = usuário)
curl localhost:8092/v1/orders -H "Authorization: Bearer $TOKEN" \
     -H 'content-type: application/json' -d '{"total_cents":1990}'
```

| Porta no host | |
|---|---|
| 8091 | users-service |
| 8092 | orders-service |
| 8280 | Keycloak (console: admin/admin) |
| 5434 | Postgres |

As portas foram escolhidas para não colidir com o compose do monolito.

## Estrutura

```
internal/platform/          compartilhado por todos os serviços
  apperr/                   erro de aplicação (Kind + Code), sem HTTP
  httpx/                    Fiber, Problem Details e o caminho de volta (resposta -> apperr)
  config/ database/ runner/ config, migrations por serviço, ciclo de vida/healthcheck
services/<servico>/
  main.go                   binário do serviço
  <servico>api/             CONTRATO PÚBLICO: DTOs, códigos de erro, client HTTP
  <servico>test/            sobe o serviço real em memória, para testes de quem o consome
  internal/                 privado: o compilador impede import por outros serviços
    domain/ app/ httpapi/ postgres/ memory/ server/
  migrations/               SQL do banco DESTE serviço
deploy/keycloak/            realm de dev
deploy/postgres/init.sql    um banco e um usuário de banco por serviço
```

### Regras de fronteira

1. Um serviço só importa de outro o pacote **`<servico>api`** (contrato). O
   resto é `internal/`, e o compilador barra.
2. **Banco por serviço**, com usuário próprio: o orders-service nem tem
   credencial para o banco do users. Os dados de outro serviço só chegam pela
   API dele.
3. `<servico>test` só pode ser usado em `_test.go` (regra `depguard`).

## Autenticação entre serviços

```
            token da alice (aud: orders-service, users-service)
pessoa ───────────────────────────────► orders-service
                                             │  s2s.Auto: quem chamou é PESSOA
                                             │  → token exchange no Keycloak
                                             ▼
            token trocado (sub: alice, aud: SÓ users-service, azp: orders-service)
                                        users-service  → políticas aplicadas à ALICE

            token do billing (client credentials)
sistema ──────────────────────────────► orders-service
                                             │  s2s.Auto: quem chamou é SISTEMA
                                             │  → client credentials do orders
                                             ▼
            token do orders-service (users:read)
                                        users-service  → políticas aplicadas ao orders
```

- **Em nome da pessoa (token exchange, RFC 8693):** o users-service vê a
  alice, não o orders. A política "a própria pessoa pode ler o perfil" e o
  provisionamento no primeiro acesso funcionam como se ela tivesse chamado
  direto. O token trocado só vale para o `users-service`: se vazar, não serve
  para chamar o orders.
- **Como o próprio serviço (client credentials):** quando quem chamou é um
  sistema, o orders responde pela chamada com a identidade dele e o papel
  `users:read`.

Os tokens de saída ficam em cache (o exchange por pessoa e audiência).
Montagem em `services/orders/main.go`:

```go
users := usersapi.NewClient(usersURL, &http.Client{
	Timeout:   5 * time.Second, // sempre: um serviço travado não pode travar o outro
	Transport: tokens.Transport(nil, "users-service", s2s.Auto),
})
```

### Keycloak: cada serviço é um client

| Client | Papel |
|---|---|
| `users-service` | resource server (só recebe); dono dos papéis `admin`, `users:read` |
| `orders-service` | recebe e chama: client credentials + token exchange; dono de `admin`, `orders:read`, `orders:write` |
| `app-web` | front-end (pessoas, PKCE); tokens com `aud` dos dois serviços |
| `billing-service` | sistema externo; `orders:read`/`orders:write` |

Para o token exchange funcionar, o `orders-service` tem
`standard.token.exchange.enabled`, um audience mapper para o `users-service`,
e precisa estar no `aud` do token de entrada (por isso o audience mapper no
`app-web`).

## Erros através da rede

O users-service responde `404` com `{"code":"users.not_found",...}`. O
`usersapi.Client` reconstrói o `*apperr.Error` com o **mesmo Code**, e como
`apperr` compara por Code, isto funciona no orders:

```go
if errors.Is(err, usersapi.ErrNotFound) {
	return domain.ErrUnknownUser.Wrap(err) // 422 para o cliente do orders
}
```

| Resposta do serviço chamado | Vira no serviço que chamou |
|---|---|
| 4xx com Problem Details (400, 403, 404, 409, 422) | o mesmo Kind, Code, mensagem e campos |
| 401 | **500**: o token que nós enviamos foi recusado, então é configuração nossa |
| 5xx, timeout, rede, corpo que não é Problem | **503** `users.unavailable` (causa no log) |

Um `403` do serviço chamado é propagado como está. Em chamadas em nome da
pessoa é o certo ("você não pode"). Em chamadas como o próprio serviço,
indica papel faltando no service account. O log mostra a cadeia completa
para distinguir os dois casos.

## Testes sem Docker

`services/orders/internal/server/server_test.go` testa a cadeia inteira:
o orders-service, o **users-service real** (`userstest.Start`, em memória
atrás de HTTP) e um "Keycloak" de teste (`authtest.TokenServer`) fazendo
client credentials e token exchange. Cobre a identidade da pessoa chegando ao
users, o cache de tokens, o users fora do ar (503) e o service account sem
papel (403).

## Docker

Um `Dockerfile` para todos: `docker build --build-arg SERVICE=users .`. Cada
imagem tem ~15 MB (distroless, binário estático), e o CI falha acima de 40 MB.

## Adicionando um serviço

1. Copie `services/orders` (ou `services/users`, se ele não chama ninguém).
2. Crie o client no Keycloak (`deploy/keycloak/realm-app.json`) com os papéis
   do serviço e audience mappers em quem vai chamá-lo.
3. Adicione o banco/usuário em `deploy/postgres/init.sql` e o serviço no
   `docker-compose.yml`, no `Makefile` (`SERVICES`) e na matriz do CI.
4. Publique o contrato em `<servico>api` e, se outros serviços forem testar
   contra ele, um `<servico>test`.

## Próximos passos comuns

Ficaram de fora para manter o template enxuto:

- tracing distribuído (OpenTelemetry), propagando trace/request ID entre serviços;
- retries com backoff e circuit breaker nas chamadas entre serviços;
- comunicação assíncrona (eventos/fila) e outbox;
- um API gateway na frente dos serviços.

## Keycloak de dev × produção

O realm é **só para desenvolvimento**: senhas iguais ao usuário, secrets
fixos e *direct access grants* ligados no `app-web` (é o que permite o
`make token`). Em produção: desligue *direct access grants*, use secrets
gerados, HTTPS e o Keycloak em modo `start` com banco próprio.
