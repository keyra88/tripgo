DATABASE_URL ?= $(shell grep DATABASE_URL .env 2>/dev/null | cut -d '=' -f2-)
ifeq ($(DATABASE_URL),)
	DATABASE_URL := postgres://tripgo:tripgo@localhost:5432/tripgo?sslmode=disable
endif

.PHONY: generate migrate migrate-down run test

generate:
	mkdir -p api
	go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-o api/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml

migrate:
	go tool goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	go tool goose -dir migrations postgres "$(DATABASE_URL)" down

run:
	go run ./cmd/trip-service

test:
	go test -v -race ./...
