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
	set -a; . ./.env; set +a; \
	psql "$$DATABASE_URL" -f migrations/000003_customer_schema.sql; \
	psql "$$DATABASE_URL" -f migrations/000004_driver_schema.sql