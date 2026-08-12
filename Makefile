.PHONY: run build tidy migrate-up

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

tidy:
	go mod tidy

# Requires: psql client installed, DATABASE_URL exported or set in .env
migrate-up:
	psql "$$DATABASE_URL" -f migrations/000001_init.sql
