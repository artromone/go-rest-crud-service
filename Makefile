include .env
export

DSN := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSLMODE)
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@latest

.PHONY: up down down-hard migrate migrate-down run test tidy

.env:
	cp .env.example .env
	@echo "Создан .env — проверьте значения перед запуском"

up: .env
	docker compose up -d --wait

down:
	docker compose down

migrate:
	$(GOOSE) -dir ./migrations postgres "$(DSN)" up

migrate-down:
	$(GOOSE) -dir ./migrations postgres "$(DSN)" down

run:
	go run ./cmd/api

test:
	go test ./... -count=1

tidy:
	go mod tidy
