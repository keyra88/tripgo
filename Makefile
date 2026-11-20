export PATH := $(CURDIR)/bin:$(PATH)

DATABASE_URL ?= $(shell grep DATABASE_URL .env 2>/dev/null | cut -d '=' -f2-)
export DATABASE_URL

.PHONY: generate migrate migrate-down run test test-integration lint docker-build

generate:
	mkdir -p bin api
	go build -o ./bin/mockgen go.uber.org/mock/mockgen
	go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-o api/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml
	go generate ./...

migrate:
	@test -n "$$DATABASE_URL" || { echo "DATABASE_URL is required"; exit 1; }
	go tool goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	@test -n "$$DATABASE_URL" || { echo "DATABASE_URL is required"; exit 1; }
	go tool goose -dir migrations postgres "$(DATABASE_URL)" down

run:
	go run ./cmd/trip-service

test:
	go test -v -race -p 1 -tags=integration -coverprofile=coverage.out -coverpkg=./internal/domain/...,./internal/usecase/... ./...
	@go tool cover -func=coverage.out

test-integration:
	go test -v -race -p 1 -tags=integration ./internal/repository/trip/... ./tests/...

lint:
	golangci-lint run

docker-build:
	docker build -f deploy/Dockerfile -t tripgo-service:local .
