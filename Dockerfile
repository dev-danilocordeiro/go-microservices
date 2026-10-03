# syntax=docker/dockerfile:1
# Um Dockerfile para todos os serviços: docker build --build-arg SERVICE=users .

# ---- build: compila um binário estático (sem cgo) ----
FROM golang:1.27-alpine AS build
ARG SERVICE
RUN test -n "$SERVICE" || (echo "use --build-arg SERVICE=<users|orders>" && false)
WORKDIR /src

# Dependências numa camada própria: só baixa de novo quando go.mod/go.sum mudam.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X github.com/dev-danilocordeiro/go-microservices/internal/platform/runner.Version=${VERSION}" \
      -o /out/service ./services/${SERVICE}

# ---- runtime: distroless static (~2 MB), sem shell, sem gerenciador de pacotes ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/service /service
USER nonroot:nonroot
EXPOSE 8080
# Sem curl/wget na imagem: o próprio binário faz o healthcheck.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/service", "healthcheck"]
ENTRYPOINT ["/service"]
