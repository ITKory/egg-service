APP_NAME ?= egg-service
BIN_DIR ?= bin
GO ?= go
PKG ?= ./...
MAIN ?= ./cmd/server
BUILD_FLAGS ?= -trimpath -ldflags="-s -w"

.DEFAULT_GOAL := help

.PHONY: help run go-run build test test-race cover fmt vet lint tidy check clean docker-up docker-down docker-logs

help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: ## Run the API locally
	$(GO) run $(MAIN)

go-run: run ## Backward-compatible alias for run

build: ## Build the production binary
	@mkdir -p $(BIN_DIR)
	$(GO) build $(BUILD_FLAGS) -o $(BIN_DIR)/$(APP_NAME) $(MAIN)

test: ## Run unit tests
	$(GO) test $(PKG)

test-race: ## Run tests with the race detector
	$(GO) test -race $(PKG)

cover: ## Run tests and generate coverage profile
	$(GO) test -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out

fmt: ## Format Go code
	$(GO) fmt $(PKG)

vet: ## Run go vet
	$(GO) vet $(PKG)

lint: ## Run golangci-lint if available, otherwise go vet
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run $(PKG); \
	else \
		$(GO) vet $(PKG); \
	fi

tidy: ## Clean up go.mod and go.sum
	$(GO) mod tidy

check: fmt vet test-race ## Run formatting, vet and race tests

clean: ## Remove build and coverage artifacts
	@rm -rf $(BIN_DIR) coverage.out cpu.out ws.test

docker-up: ## Start local dependencies and service
	docker compose up -d --build

docker-down: ## Stop local compose stack
	docker compose down

docker-logs: ## Follow backend logs
	docker compose logs -f backend
