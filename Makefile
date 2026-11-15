export PATH := $(CURDIR)/bin:$(PATH)

DATABASE_URL ?= $(shell grep DATABASE_URL .env 2>/dev/null | cut -d '=' -f2-)
ifeq ($(DATABASE_URL),)
	DATABASE_URL := postgres://tripgo:tripgo@localhost:22032/tripgo?sslmode=disable
endif

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
	go tool goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	go tool goose -dir migrations postgres "$(DATABASE_URL)" down

run:
	go run ./cmd/trip-service

test:
	go test -v -race -coverprofile=coverage.out -coverpkg=./internal/domain/...,./internal/usecase/... ./internal/domain/... ./internal/usecase/... ./internal/http/... ./internal/config/...
	@go tool cover -func=coverage.out

test-integration:
	go test -v -race -tags=integration ./internal/repository/trip/... ./tests/...

lint:
	golangci-lint run

docker-build:
	docker build -f deploy/Dockerfile -t tripgo-service:local .
