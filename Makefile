.PHONY: test lint up down run-users run-orders docker-build docker-up docker-down token token-service tidy

KC_ISSUER   := http://localhost:8280/realms/app
KC_TOKEN_URL := $(KC_ISSUER)/protocol/openid-connect/token
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
SERVICES := users orders

test: ## não precisa de nada rodando: o users-service sobe em memória nos testes do orders
	go test -race ./...

lint:
	go vet ./...
	golangci-lint run

up: ## Postgres + Keycloak
	docker compose up -d --wait

down:
	docker compose --profile app down

run-users: ## users-service em :8091
	HTTP_ADDR=:8091 OIDC_ISSUER_URL=$(KC_ISSUER) \
	DATABASE_URL='postgres://users_svc:users@localhost:5434/users?sslmode=disable' \
	go run ./services/users

run-orders: ## orders-service em :8092 (precisa do users-service rodando)
	HTTP_ADDR=:8092 OIDC_ISSUER_URL=$(KC_ISSUER) OIDC_CLIENT_SECRET=orders-secret \
	USERS_URL=http://localhost:8091 \
	DATABASE_URL='postgres://orders_svc:orders@localhost:5434/orders?sslmode=disable' \
	go run ./services/orders

docker-build: ## imagem de cada serviço, com o tamanho
	@for s in $(SERVICES); do \
		docker build -q --build-arg SERVICE=$$s --build-arg VERSION=$(VERSION) -t go-microservices-$$s:latest . >/dev/null && \
		docker image ls go-microservices-$$s:latest --format "$$s: {{.Size}}"; \
	done

docker-up: ## tudo em containers
	docker compose --profile app up -d --build --wait

docker-down:
	docker compose --profile app down

U ?= alice
token: ## token de pessoa: make token U=alice|bob|admin (senha = usuário)
	@curl -s $(KC_TOKEN_URL) -d grant_type=password -d client_id=app-web \
		-d username=$(U) -d password=$(U) | python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])'

token-service: ## token do billing-service (client credentials)
	@curl -s $(KC_TOKEN_URL) -d grant_type=client_credentials -d client_id=billing-service \
		-d client_secret=billing-secret | python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])'

tidy:
	go mod tidy
