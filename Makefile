.DEFAULT_GOAL := help

# Local Postgres/Redis default connection info (overridable via env).
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/urlshortener?sslmode=disable
MIGRATIONS_DIR := migrations

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: run
run: ## Run the server locally
	go run ./cmd/server

.PHONY: build
build: ## Build the server binary into ./bin
	go build -o bin/server ./cmd/server

.PHONY: test
test: ## Run unit tests (skips integration-tagged tests)
	go test -race -count=1 ./...

.PHONY: test-integration
test-integration: ## Run integration tests (needs Docker for testcontainers)
	go test -race -count=1 -tags=integration ./...

.PHONY: cover
cover: ## Run unit tests with coverage report
	go test -race -count=1 -coverprofile=coverage.txt ./...
	go tool cover -func=coverage.txt | tail -1

.PHONY: lint
lint: ## Run golangci-lint (zero errors required)
	golangci-lint run ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: bench
bench: ## Run redirect hot-path benchmarks
	go test -bench=. -benchmem -run=^$$ ./internal/service/... ./internal/handler/...

.PHONY: migrate-up
migrate-up: ## Apply all up migrations
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

.PHONY: migrate-create
migrate-create: ## Create a new migration: make migrate-create name=add_foo
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

.PHONY: compose-up
compose-up: ## Start Postgres + Redis + app via docker compose
	docker compose up --build

.PHONY: compose-down
compose-down: ## Stop and remove docker compose stack
	docker compose down -v

.PHONY: tidy
tidy: ## Tidy go modules
	go mod tidy
