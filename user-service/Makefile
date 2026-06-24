USER_SERVICE_CONFIG ?= ./configs/config.yaml
USER_SERVICE_BIN ?= ../bin/user-service
ADMIN_USERNAME ?= admin
ADMIN_NICKNAME ?= Admin
ADMIN_RESET_PASSWORD ?= false

.PHONY: help build run test lint architecture-lint verify seed-rbac create-super-admin generate migrate-diff migrate-validate migrate-apply openapi-generate fxgraph-generate fxgraph-check

help: ## 查看 user-service 可用命令。
	@awk 'BEGIN {FS = ":.*##"; printf "user-service 可用命令：\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## 构建 user-service 二进制。
	@mkdir -p $(dir $(USER_SERVICE_BIN))
	go build -o $(USER_SERVICE_BIN) ./cmd

run: ## 使用 USER_SERVICE_CONFIG 运行 user-service。
	go run ./cmd serve --config $(USER_SERVICE_CONFIG)

test: ## 运行 user-service 测试。
	go test ./...

lint: ## 运行 user-service lint。
	golangci-lint run ./...

architecture-lint: ## 运行 user-service 架构边界检查。
	./scripts/architecture-lint.sh

verify: lint architecture-lint test openapi-generate ## 运行 user-service 验证。
	git diff --exit-code

seed-rbac: ## 使用 USER_SERVICE_CONFIG 初始化 RBAC 系统数据。
	go run ./cmd rbac --config $(USER_SERVICE_CONFIG) seed

create-super-admin: ## 创建管理员用户并绑定超级管理员角色；需要 ADMIN_PASSWORD 环境变量。
	@test -n "$${ADMIN_PASSWORD}" || (echo "必须设置 ADMIN_PASSWORD" >&2; exit 2)
	ADMIN_PASSWORD="$${ADMIN_PASSWORD}" go run ./cmd rbac --config $(USER_SERVICE_CONFIG) create-super-admin --username '$(ADMIN_USERNAME)' --nickname '$(ADMIN_NICKNAME)' --reset-password=$(ADMIN_RESET_PASSWORD)

generate: ## 生成 user-service Ent 代码。
	go generate ./ent

migrate-diff: ## 生成 migration，需传入 name=<migration-name>。
	@test -n "$(name)" || (echo "用法：make migrate-diff name=<migration-name>" >&2; exit 2)
	./scripts/migrate-diff.sh "$(name)"

migrate-validate: ## 校验 migrations。
	./scripts/migrate-validate.sh

migrate-apply: ## 使用 DATABASE_URL 执行 migrations。
	./scripts/migrate-apply.sh

openapi-generate: ## 生成 OpenAPI 3 文档。
	./scripts/openapi-generate.sh

fxgraph-generate: ## 生成 user-service Fx 依赖图。
	go run ./cmd fxgraph --config $(USER_SERVICE_CONFIG) --output ./docs/fx-dependency-graph.dot

fxgraph-check: fxgraph-generate ## 检查 user-service Fx 依赖图是否存在 drift。
	git diff --exit-code -- docs/fx-dependency-graph.dot
