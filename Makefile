PLATFORM_DIR := platform
TOOLS_OPENAPI_CONVERT_DIR := tools/openapi-convert
TOOLS_NACOS_CONFIG_SEED_DIR := tools/nacos-config-seed
TOOLS_SERVICE_SCAFFOLD_DIR := tools/service-scaffold
# 根目录业务服务清单。新增服务时追加目录名，并同步登记 go.work。
BUSINESS_SERVICES := user-service
USER_SERVICE_DIR := user-service
USER_SERVICE_BIN ?= $(CURDIR)/bin/user-service
ADMIN_USERNAME ?=
ADMIN_NICKNAME ?=
NACOS_ADDR ?= 127.0.0.1:8848
NACOS_GROUP ?= AEGISCORE
NACOS_TIMEOUT ?= 30s
LOCAL_NACOS_NAMESPACE ?= loca-host
LOCAL_NACOS_USERNAME ?= nacos
LOCAL_NACOS_PASSWORD ?= nacos
# 仅供仓库根目录的本地进程入口使用。已导出的 AEGISCORE_NACOS_* 优先，服务 Makefile 不拥有这些默认值。
LOCAL_NACOS_ENV = \
	AEGISCORE_SERVICE=$${AEGISCORE_SERVICE:-user-service} \
	AEGISCORE_NACOS_ADDR=$${AEGISCORE_NACOS_ADDR:-$(NACOS_ADDR)} \
	AEGISCORE_NACOS_NAMESPACE=$${AEGISCORE_NACOS_NAMESPACE:-$(LOCAL_NACOS_NAMESPACE)} \
	AEGISCORE_NACOS_GROUP=$${AEGISCORE_NACOS_GROUP:-$(NACOS_GROUP)} \
	AEGISCORE_NACOS_USERNAME=$${AEGISCORE_NACOS_USERNAME:-$(LOCAL_NACOS_USERNAME)} \
	AEGISCORE_NACOS_PASSWORD=$${AEGISCORE_NACOS_PASSWORD:-$(LOCAL_NACOS_PASSWORD)}
export ADMIN_BOOTSTRAP_PASSWORD

.PHONY: help build test test-containers coverage lint verify
.PHONY: platform-test platform-test-containers platform-lint platform-generate platform-verify
.PHONY: tools-openapi-convert-test tools-openapi-convert-lint tools-nacos-config-seed-test tools-nacos-config-seed-lint tools-nacos-config-seed-local-host tools-nacos-config-seed-local-docker tools-service-scaffold-test tools-service-scaffold-lint new-service
.PHONY: user-service-build user-service-run user-service-test user-service-test-containers user-service-rbac-sync-race-test user-service-lint user-service-verify user-service-architecture-lint
.PHONY: user-service-seed-rbac user-service-bootstrap-super-admin user-service-image-verify
.PHONY: user-service-generate user-service-migrate-diff user-service-migrate-validate user-service-openapi-generate user-service-fxgraph-generate user-service-fxgraph-check
.PHONY: compose-dashboard-generate compose-dashboard-check local-init delivery-verify github-actions-pins-check

help: ## 查看可用命令。
	@awk 'BEGIN {FS = ":.*##"; printf "可用命令：\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-36s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: $(addsuffix -build,$(BUSINESS_SERVICES)) ## 构建全部已登记业务服务二进制。

test: platform-test $(addsuffix -test,$(BUSINESS_SERVICES)) tools-openapi-convert-test tools-nacos-config-seed-test tools-service-scaffold-test ## 运行全部 Go 模块测试。

test-containers: platform-test-containers $(addsuffix -test-containers,$(BUSINESS_SERVICES)) ## 运行 platform 与已登记业务服务的全部 Docker-backed 测试。

coverage: ## 生成覆盖率报告并检查手写关键包基线。
	./scripts/check-coverage.sh

lint: platform-lint $(addsuffix -lint,$(BUSINESS_SERVICES)) tools-openapi-convert-lint tools-nacos-config-seed-lint tools-service-scaffold-lint ## 运行全部 Go 模块 lint。

verify: lint $(addsuffix -architecture-lint,$(BUSINESS_SERVICES)) github-actions-pins-check delivery-verify platform-generate $(addsuffix -generate,$(BUSINESS_SERVICES)) test $(addsuffix -openapi-generate,$(BUSINESS_SERVICES)) ## 运行完整本地验证。
	@if [ -e .git ]; then git diff --exit-code -- . ':(exclude)AGENTS.md' ':(exclude)openspec/AGENTS.md' ':(exclude)CLAUDE.md' ':(exclude).multica/project/resources.json' ':(exclude).multica/**'; \
	else echo "git diff: skipped (project has no .git)"; fi

github-actions-pins-check: ## 检查 GitHub Actions 第三方 Action 是否固定 commit SHA。
	./scripts/check-github-actions-pins.sh

platform-test: ## 运行 platform 模块测试。
	$(MAKE) -C $(PLATFORM_DIR) test

platform-test-containers: ## 运行 platform Docker-backed 测试。
	$(MAKE) -C $(PLATFORM_DIR) test-containers

platform-lint: ## 运行 platform 模块 lint。
	$(MAKE) -C $(PLATFORM_DIR) lint

platform-generate: ## 生成 platform Go 生成物。
	$(MAKE) -C $(PLATFORM_DIR) generate

platform-verify: ## 运行 platform 模块验证。
	$(MAKE) -C $(PLATFORM_DIR) verify

tools-openapi-convert-test: ## 运行 OpenAPI 转换工具测试。
	$(MAKE) -C $(TOOLS_OPENAPI_CONVERT_DIR) test

tools-openapi-convert-lint: ## 运行 OpenAPI 转换工具 lint。
	$(MAKE) -C $(TOOLS_OPENAPI_CONVERT_DIR) lint

tools-nacos-config-seed-test: ## 运行 Nacos 配置初始化工具测试。
	$(MAKE) -C $(TOOLS_NACOS_CONFIG_SEED_DIR) test

tools-nacos-config-seed-lint: ## 运行 Nacos 配置初始化工具 lint。
	$(MAKE) -C $(TOOLS_NACOS_CONFIG_SEED_DIR) lint

tools-nacos-config-seed-local-host: ## 发布宿主机进程使用的本地 Nacos 配置，可覆盖 NACOS_ADDR/NACOS_GROUP/NACOS_TIMEOUT。
	go run ./$(TOOLS_NACOS_CONFIG_SEED_DIR) \
		-addr '$(NACOS_ADDR)' \
		-namespace loca-host \
		-group '$(NACOS_GROUP)' \
		-config-dir deployments/nacos/local-host \
		-timeout '$(NACOS_TIMEOUT)'

tools-nacos-config-seed-local-docker: ## 发布 Compose 容器使用的本地 Nacos 配置，可覆盖 NACOS_ADDR/NACOS_GROUP/NACOS_TIMEOUT。
	go run ./$(TOOLS_NACOS_CONFIG_SEED_DIR) \
		-addr '$(NACOS_ADDR)' \
		-namespace loca-docker \
		-group '$(NACOS_GROUP)' \
		-config-dir deployments/nacos/local-docker \
		-timeout '$(NACOS_TIMEOUT)'

tools-service-scaffold-test: ## 运行服务脚手架工具测试。
	$(MAKE) -C $(TOOLS_SERVICE_SCAFFOLD_DIR) test

tools-service-scaffold-lint: ## 运行服务脚手架工具 lint。
	$(MAKE) -C $(TOOLS_SERVICE_SCAFFOLD_DIR) lint

new-service: ## 使用脚手架生成新业务服务骨架，用法：make new-service name=<service-name> [port=<port>]
	@test -n "$(name)" || (echo "用法：make new-service name=<service-name> [port=<port>]" >&2; exit 2)
	go run ./$(TOOLS_SERVICE_SCAFFOLD_DIR) -name '$(name)' $(if $(port),-port '$(port)',)

%-build:
	$(MAKE) -C $* build

%-run:
	$(LOCAL_NACOS_ENV) AEGISCORE_SERVICE=$* $(MAKE) -C $* run

%-test:
	$(MAKE) -C $* test

%-lint:
	$(MAKE) -C $* lint

%-verify:
	$(MAKE) -C $* verify

user-service-build: ## 构建 user-service 二进制。
	$(MAKE) -C $(USER_SERVICE_DIR) build USER_SERVICE_BIN='$(USER_SERVICE_BIN)'

user-service-run: ## 使用本地 Nacos API 运行 user-service；已设置的 AEGISCORE_NACOS_* 优先。
	$(LOCAL_NACOS_ENV) $(MAKE) -C $(USER_SERVICE_DIR) run

user-service-test: ## 运行 user-service 测试。
	$(MAKE) -C $(USER_SERVICE_DIR) test

user-service-test-containers: ## 运行 user-service Docker-backed 测试。
	$(MAKE) -C $(USER_SERVICE_DIR) test-containers

user-service-rbac-sync-race-test: ## 使用 race detector 运行 user-service RBAC policy sync 相关测试。
	$(MAKE) -C $(USER_SERVICE_DIR) rbac-sync-race-test

user-service-lint: ## 运行 user-service lint。
	$(MAKE) -C $(USER_SERVICE_DIR) lint

user-service-verify: ## 运行 user-service 验证。
	$(MAKE) -C $(USER_SERVICE_DIR) verify

user-service-architecture-lint: ## 运行 user-service 架构边界检查。
	$(MAKE) -C $(USER_SERVICE_DIR) architecture-lint

user-service-seed-rbac: ## 使用本地 Nacos API 初始化 user-service RBAC 系统数据；已设置的 AEGISCORE_NACOS_* 优先。
	$(LOCAL_NACOS_ENV) $(MAKE) -C $(USER_SERVICE_DIR) seed-rbac

user-service-bootstrap-super-admin: ## 为全新数据库一次性创建初始超级管理员；密码来自 ADMIN_BOOTSTRAP_PASSWORD 环境变量。
	$(LOCAL_NACOS_ENV) $(MAKE) -C $(USER_SERVICE_DIR) bootstrap-super-admin ADMIN_USERNAME='$(ADMIN_USERNAME)' ADMIN_NICKNAME='$(ADMIN_NICKNAME)'

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

local-init: ## 显式执行本地 Nacos 管理员校验、两套配置发布和 RBAC seed；成功容器自动删除。
	docker compose -f deployments/compose/docker-compose.yml --profile init run --rm nacos-init-auth
	docker compose -f deployments/compose/docker-compose.yml --profile init run --rm nacos-init-host
	docker compose -f deployments/compose/docker-compose.yml --profile init run --rm nacos-init-docker
	docker compose -f deployments/compose/docker-compose.yml restart user-service
	docker compose -f deployments/compose/docker-compose.yml --profile init run --rm rbac-seed

delivery-verify: ## 校验 Helm、Kustomize、Compose、Prometheus 和 dashboard 交付资产。
	./scripts/verify-delivery-assets.sh
