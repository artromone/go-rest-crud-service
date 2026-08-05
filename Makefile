# Переменные подтягиваются из .env — того же файла, что читают
# docker compose и приложение. Ни одного пароля в самом Makefile.
include .env
export

DSN := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSLMODE)
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@latest

.PHONY: up down down-hard migrate migrate-down run test tidy

## .env: создаётся из примера, если его ещё нет
.env:
	cp .env.example .env
	@echo "Создан .env — проверьте значения перед запуском"

## up: поднять postgres и дождаться, пока он реально готов (--wait)
up: .env
	docker compose up -d --wait

## down: остановить postgres. Данные остаются в volume.
down:
	docker compose down

## down-hard: остановить и стереть данные вместе с volume
down-hard:
	docker compose down -v

## migrate: применить все миграции
migrate:
	$(GOOSE) -dir ./migrations postgres "$(DSN)" up

## migrate-down: откатить последнюю миграцию
migrate-down:
	$(GOOSE) -dir ./migrations postgres "$(DSN)" down

## run: запустить сервис
run:
	go run ./cmd/api

## test: прогнать тесты (база не нужна)
test:
	go test ./... -count=1

tidy:
	go mod tidy
