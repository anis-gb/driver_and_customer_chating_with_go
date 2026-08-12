.PHONY: run build tidy migrate-up watch

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

tidy:
	go mod tidy

watch:
	air

migrate-up:
	psql "$$DATABASE_URL" -f migrations/000001_init.sql
