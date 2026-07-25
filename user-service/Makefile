USER_SERVICE_BIN ?= ../bin/user-service
ADMIN_USERNAME ?=
ADMIN_NICKNAME ?=
export ADMIN_BOOTSTRAP_PASSWORD

.PHONY: help build run test test-containers lint architecture-lint architecture-lint-test verify seed-rbac bootstrap-super-admin generate migrate-diff migrate-validate openapi-generate fxgraph-generate fxgraph-check

help: ## 查看 user-service 可用命令。
	@awk 'BEGIN {FS = ":.*##"; printf "user-service 可用命令：\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## 构建 user-service 二进制。
	@mkdir -p $(dir $(USER_SERVICE_BIN))
	go build -o $(USER_SERVICE_BIN) ./cmd

run: ## 使用 AEGISCORE_NACOS_* 环境变量运行 user-service。
	go run ./cmd serve

test: ## 运行 user-service 测试。
	go test ./...

test-containers: ## 运行包含 Docker-backed 容器的 user-service 测试。
	go test ./... -args -aegiscore.testcontainers

lint: ## 运行 user-service lint。
	golangci-lint run ./...

architecture-lint: architecture-lint-test ## 运行 user-service 架构边界检查。
	./scripts/architecture-lint.sh

architecture-lint-test: ## 使用故意违规 fixture 验证架构检查可命中分层污染。
	./scripts/architecture-lint-test.sh

verify: lint architecture-lint generate test openapi-generate ## 运行 user-service 验证。
	git diff --exit-code

seed-rbac: ## 使用 AEGISCORE_NACOS_* 环境变量初始化 RBAC 系统数据。
	go run ./cmd rbac seed

bootstrap-super-admin: ## 一次性创建初始超级管理员；密码来自 ADMIN_BOOTSTRAP_PASSWORD 环境变量。
	@test -n "$(ADMIN_USERNAME)" || (echo "必须设置 ADMIN_USERNAME" >&2; exit 2)
	@test -n "$${ADMIN_BOOTSTRAP_PASSWORD}" || (echo "必须设置 ADMIN_BOOTSTRAP_PASSWORD" >&2; exit 2)
	go run ./cmd rbac bootstrap-super-admin --username '$(ADMIN_USERNAME)' --nickname '$(ADMIN_NICKNAME)' --password-env ADMIN_BOOTSTRAP_PASSWORD

generate: ## 生成 user-service Go 生成物。
	go generate ./...

migrate-diff: ## 生成 migration，需传入 name=<migration-name>。
	@test -n "$(name)" || (echo "用法：make migrate-diff name=<migration-name>" >&2; exit 2)
	./scripts/migrate-diff.sh "$(name)"

migrate-validate: ## 校验 migrations。
	./scripts/migrate-validate.sh

openapi-generate: ## 生成 OpenAPI 3 文档。
	./scripts/openapi-generate.sh

fxgraph-generate: ## 生成 user-service Fx 依赖图。
	go run ./cmd fxgraph --output ./docs/fx-dependency-graph.dot

fxgraph-check: fxgraph-generate ## 检查 user-service Fx 依赖图是否存在 drift。
	git diff --exit-code -- docs/fx-dependency-graph.dot
