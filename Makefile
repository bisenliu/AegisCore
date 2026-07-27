COMMON_DIR := common
TOOLS_OPENAPI_CONVERT_DIR := tools/openapi-convert
TOOLS_NACOS_CONFIG_SEED_DIR := tools/nacos-config-seed
USER_SERVICE_DIR := user-service
USER_SERVICE_BIN ?= $(CURDIR)/bin/user-service
ADMIN_USERNAME ?=
ADMIN_NICKNAME ?=
export ADMIN_BOOTSTRAP_PASSWORD

.PHONY: help build test lint verify
.PHONY: common-test common-lint common-generate common-verify
.PHONY: tools-openapi-convert-test tools-nacos-config-seed-test
.PHONY: user-service-build user-service-run user-service-test user-service-lint user-service-verify user-service-architecture-lint
.PHONY: user-service-seed-rbac user-service-bootstrap-super-admin user-service-image-verify
.PHONY: user-service-generate user-service-migrate-diff user-service-migrate-validate user-service-openapi-generate user-service-fxgraph-generate user-service-fxgraph-check
.PHONY: compose-dashboard-generate compose-dashboard-check

help: ## 查看可用命令。
	@awk 'BEGIN {FS = ":.*##"; printf "可用命令：\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-36s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: user-service-build ## 构建全部服务二进制。

test: common-test user-service-test tools-openapi-convert-test tools-nacos-config-seed-test ## 运行全部 Go 模块测试。

lint: common-lint user-service-lint ## 运行全部 Go 模块 lint。

verify: lint user-service-architecture-lint common-generate user-service-generate test user-service-openapi-generate ## 运行完整本地验证。
	git diff --exit-code -- . ':(exclude)AGENTS.md' ':(exclude)openspec/AGENTS.md' ':(exclude)CLAUDE.md' ':(exclude).multica/project/resources.json' ':(exclude).multica/**'

common-test: ## 运行 common 模块测试。
	$(MAKE) -C $(COMMON_DIR) test

common-lint: ## 运行 common 模块 lint。
	$(MAKE) -C $(COMMON_DIR) lint

common-generate: ## 生成 common Go 生成物。
	$(MAKE) -C $(COMMON_DIR) generate

common-verify: ## 运行 common 模块验证。
	$(MAKE) -C $(COMMON_DIR) verify

tools-openapi-convert-test: ## 运行 OpenAPI 转换工具测试。
	$(MAKE) -C $(TOOLS_OPENAPI_CONVERT_DIR) test

tools-nacos-config-seed-test: ## 运行 Nacos 配置初始化工具测试。
	$(MAKE) -C $(TOOLS_NACOS_CONFIG_SEED_DIR) test

user-service-build: ## 构建 user-service 二进制。
	$(MAKE) -C $(USER_SERVICE_DIR) build USER_SERVICE_BIN='$(USER_SERVICE_BIN)'

user-service-run: ## 使用 AEGISCORE_NACOS_* 环境变量运行 user-service。
	$(MAKE) -C $(USER_SERVICE_DIR) run

user-service-test: ## 运行 user-service 测试。
	$(MAKE) -C $(USER_SERVICE_DIR) test

user-service-lint: ## 运行 user-service lint。
	$(MAKE) -C $(USER_SERVICE_DIR) lint

user-service-verify: ## 运行 user-service 验证。
	$(MAKE) -C $(USER_SERVICE_DIR) verify

user-service-architecture-lint: ## 运行 user-service 架构边界检查。
	$(MAKE) -C $(USER_SERVICE_DIR) architecture-lint

user-service-seed-rbac: ## 使用 AEGISCORE_NACOS_* 环境变量初始化 user-service RBAC 系统数据。
	$(MAKE) -C $(USER_SERVICE_DIR) seed-rbac

user-service-bootstrap-super-admin: ## 为全新数据库一次性创建初始超级管理员；密码来自 ADMIN_BOOTSTRAP_PASSWORD 环境变量。
	$(MAKE) -C $(USER_SERVICE_DIR) bootstrap-super-admin ADMIN_USERNAME='$(ADMIN_USERNAME)' ADMIN_NICKNAME='$(ADMIN_NICKNAME)'

user-service-image-verify: ## 校验 user-service Distroless 镜像内容。
	./deployments/docker/verify-user-service-image.sh aegiscore-user-service:latest

user-service-generate: ## 生成 user-service Go 生成物。
	$(MAKE) -C $(USER_SERVICE_DIR) generate

user-service-migrate-diff: ## 生成 user-service migration，需传入 name=<migration-name>。
	$(MAKE) -C $(USER_SERVICE_DIR) migrate-diff name='$(name)'

user-service-migrate-validate: ## 校验 user-service migrations。
	$(MAKE) -C $(USER_SERVICE_DIR) migrate-validate

user-service-openapi-generate: ## 生成 user-service OpenAPI 3 文档。
	$(MAKE) -C $(USER_SERVICE_DIR) openapi-generate

user-service-fxgraph-generate: ## 生成 user-service Fx 依赖图。
	$(MAKE) -C $(USER_SERVICE_DIR) fxgraph-generate

user-service-fxgraph-check: ## 检查 user-service Fx 依赖图是否存在 drift。
	$(MAKE) -C $(USER_SERVICE_DIR) fxgraph-check

compose-dashboard-generate: ## 从通用观测 dashboard 生成 Compose Grafana dashboard。
	./deployments/compose/scripts/generate-grafana-dashboard.sh

compose-dashboard-check: ## 检查 Compose Grafana dashboard 是否已生成且无 drift。
	./deployments/compose/scripts/generate-grafana-dashboard.sh --check
