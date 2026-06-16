COMMON_DIR := common
USER_SERVICE_DIR := user-service
USER_SERVICE_CONFIG ?= ./user-service/configs/config.yaml
USER_SERVICE_BIN ?= ./bin/user-service

.PHONY: help build build-user-service test test-common test-user-service lint lint-common lint-user-service architecture-lint verify run-user-service seed-rbac generate migrate-diff migrate-validate migrate-apply openapi-generate

help: ## Show available commands.
	@awk 'BEGIN {FS = ":.*##"; printf "Available commands:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: build-user-service ## Build all service binaries.

build-user-service: ## Build the user-service binary.
	@mkdir -p $(dir $(USER_SERVICE_BIN))
	go build -o $(USER_SERVICE_BIN) ./$(USER_SERVICE_DIR)/cmd

test: test-common test-user-service ## Run tests for all Go modules.

test-common: ## Run common module tests.
	cd $(COMMON_DIR) && go test ./...

test-user-service: ## Run user-service module tests.
	cd $(USER_SERVICE_DIR) && go test ./...

lint: lint-common lint-user-service ## Run lint for all Go modules.

lint-common: ## Run common module lint.
	cd $(COMMON_DIR) && golangci-lint run ./...

lint-user-service: ## Run user-service module lint.
	cd $(USER_SERVICE_DIR) && golangci-lint run ./...

architecture-lint: ## Run user-service architecture boundary checks.
	cd $(USER_SERVICE_DIR) && ./scripts/architecture-lint.sh

verify: lint architecture-lint test openapi-generate ## Run full local verification.
	git diff --exit-code

run-user-service: ## Run user-service with USER_SERVICE_CONFIG.
	go run ./$(USER_SERVICE_DIR)/cmd serve --config $(USER_SERVICE_CONFIG)

seed-rbac: ## Seed user-service RBAC data with USER_SERVICE_CONFIG.
	go run ./$(USER_SERVICE_DIR)/cmd rbac --config $(USER_SERVICE_CONFIG) seed

generate: ## Generate user-service Ent code.
	cd $(USER_SERVICE_DIR) && go generate ./ent

migrate-diff: ## Generate a user-service migration with name=<migration-name>.
	@test -n "$(name)" || (echo "Usage: make migrate-diff name=<migration-name>" >&2; exit 2)
	cd $(USER_SERVICE_DIR) && ./scripts/migrate-diff.sh "$(name)"

migrate-validate: ## Validate user-service migrations.
	cd $(USER_SERVICE_DIR) && ./scripts/migrate-validate.sh

migrate-apply: ## Apply user-service migrations using DATABASE_URL.
	cd $(USER_SERVICE_DIR) && ./scripts/migrate-apply.sh

openapi-generate: ## Generate user-service OpenAPI 3 documentation.
	cd $(USER_SERVICE_DIR) && ./scripts/openapi-generate.sh
