# AGENTS.md

Instruções para agentes de código (incluindo a linha de execução do Sentinela) que trabalham
neste repositório. Os comandos abaixo são os que o CI (`.github/workflows/ci.yml`) roda, e todos
passam na `main` num clone limpo.

## Setup

- Go na versão do `go.mod` (`go 1.27.0`; o CI usa `go-version-file: go.mod`).
- `golangci-lint` v2 (o CI usa a `latest`; config em `.golangci.yml`).
- Docker só para construir as imagens ou subir Postgres e Keycloak. Os testes não precisam.

## Build, testes e lint

```bash
go build ./...
go vet ./...
go test -race ./...      # = make test; não precisa de nada rodando
golangci-lint run        # make lint roda go vet + golangci-lint
```

- Os testes do orders sobem o users-service real em memória (`userstest.Start`) e um Keycloak
  de teste (`authtest.TokenServer`). Use `-count=1` para rodar de novo sem cache.
- O lint inclui `errorlint`, `wrapcheck`, `bodyclose`, `noctx`, `revive` e `depguard`
  (`<servico>test` e `authtest` só em `_test.go`).

## Imagens

```bash
make docker-build        # uma imagem por serviço (users, orders), com o tamanho
```

O CI falha se uma imagem passar de **40 MB** (hoje ~27 MB). Não adicione nada ao estágio final.

## Rodando localmente

```bash
make up                  # Postgres (5434) + Keycloak (8280)
make run-users           # :8091
make run-orders          # :8092, precisa do users-service rodando
```

## Regras de fronteira

1. Um serviço só importa de outro o pacote `<servico>api` (contrato); o resto é `internal/`.
2. Banco por serviço, com usuário próprio. Dados de outro serviço só chegam pela API dele.
3. Toda chamada entre serviços tem timeout no `http.Client`.

## Banco

As migrations ficam em `services/<servico>/migrations/` (`000NN_*.sql`, embutidas pelo
`embed.go`). Só para frente: crie um arquivo novo e nunca edite uma migration que já está na
`main`.

## Não mexa

A não ser que a issue peça explicitamente:

- `.github/` (CI), `Dockerfile`, `docker-compose.yml`, `deploy/keycloak/` e `deploy/postgres/`;
- migrations que já estão na `main`;
- a versão do Go e as dependências no `go.mod`/`go.sum`: dependência nova é decisão de uma
  pessoa.
