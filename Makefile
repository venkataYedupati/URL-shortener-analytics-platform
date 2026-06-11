.PHONY: test fmt api worker web-install web-build compose-up compose-down migrate

test:
	go test ./...

fmt:
	go fmt ./...
	npm --prefix web run lint

api:
	go run ./cmd/api

worker:
	go run ./cmd/worker

web-install:
	npm --prefix web install

web-build:
	npm --prefix web run build

compose-up:
	docker compose up --build

compose-down:
	docker compose down --remove-orphans

migrate:
	psql "$$POSTGRES_DSN" -f migrations/001_init.sql
