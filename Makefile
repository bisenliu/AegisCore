# ==============================================================================
# AegisCore Monorepo 统一总控 Makefile
# ==============================================================================

# 目录常量定义
PLATFORM_DIR := platform
TOOLS_OPENAPI_CONVERT_DIR := tools/openapi-convert
TOOLS_NACOS_CONFIG_SEED_DIR := tools/nacos-config-seed
TOOLS_SERVICE_SCAFFOLD_DIR := tools/service-scaffold

# 根目录业务服务清单。新增服务时在此追加目录名（如：user-service order-service），并同步登记 go.work。
BUSINESS_SERVICES := user-service

# 初始超级管理员引导参数（按需通过环境变量传入）
ADMIN_USERNAME ?=
ADMIN_NICKNAME ?=
export ADMIN_BOOTSTRAP_PASSWORD

# 本地 Nacos 默认配置（可通过环境变量覆盖）
NACOS_ADDR ?= 127.0.0.1:8848
NACOS_GROUP ?= AEGISCORE
NACOS_TIMEOUT ?= 30s
LOCAL_NACOS_NAMESPACE ?= loca-host
LOCAL_NACOS_USERNAME ?= nacos
LOCAL_NACOS_PASSWORD ?= nacos

# 本地进程启动环境变量注入模板：$(call LOCAL_NACOS_ENV,<service-name>)
# 自动将 AEGISCORE_SERVICE 绑定为当前目标微服务名；外部如果显式导出了同名环境变量，则保持外部优先。
LOCAL_NACOS_ENV = \
	AEGISCORE_SERVICE="$(or $(AEGISCORE_SERVICE),$(if $(1),$(1),user-service))" \
	AEGISCORE_NACOS_ADDR="$(or $(AEGISCORE_NACOS_ADDR),$(NACOS_ADDR))" \
	AEGISCORE_NACOS_NAMESPACE="$(or $(AEGISCORE_NACOS_NAMESPACE),$(LOCAL_NACOS_NAMESPACE))" \
	AEGISCORE_NACOS_GROUP="$(or $(AEGISCORE_NACOS_GROUP),$(NACOS_GROUP))" \
	AEGISCORE_NACOS_USERNAME="$(or $(AEGISCORE_NACOS_USERNAME),$(LOCAL_NACOS_USERNAME))" \
	AEGISCORE_NACOS_PASSWORD="$(or $(AEGISCORE_NACOS_PASSWORD),$(LOCAL_NACOS_PASSWORD))"

# 声明伪目标，避免与同名文件产生冲突
.PHONY: help build test test-containers coverage lint verify
.PHONY: platform-test platform-test-containers platform-lint platform-generate platform-verify
.PHONY: tools-openapi-convert-test tools-openapi-convert-lint tools-nacos-config-seed-test tools-nacos-config-seed-lint
.PHONY: tools-nacos-config-seed-local-host tools-nacos-config-seed-local-docker tools-service-scaffold-test tools-service-scaffold-lint new-service
.PHONY: user-service-rbac-sync-race-test user-service-seed-rbac user-service-bootstrap-super-admin
.PHONY: user-service-image-verify user-service-migrate-diff user-service-migrate-validate
.PHONY: user-service-fxgraph-generate user-service-fxgraph-check
.PHONY: compose-dashboard-generate compose-dashboard-check local-init delivery-verify github-actions-pins-check

# ------------------------------------------------------------------------------
# 帮助信息
# ------------------------------------------------------------------------------

help: ## 查看仓库全部可用命令与分类指引。
	@awk -v services="$(BUSINESS_SERVICES)" 'BEGIN {FS = ":.*##"; printf "\nAegisCore 可用指令清单：\n  make \033[36m<目标>\033[0m\n"} \
		/^##@/ { title = substr($$0, 5); if (title ~ /^业务微服务通用生命周期$$/) title = title " (已登记服务: " services ")"; printf "\n\033[1;33m%s\033[0m\n", title } \
		/^[a-zA-Z0-9_-]+:.*##/ { printf "  \033[36m%-36s\033[0m %s\n", $$1, $$2 } \
		/^##%SERVICES_PLACEHOLDER%/ { \
			n = split(services, svcs, " "); \
			for (i = 1; i <= n; i++) { \
				s = svcs[i]; \
				printf "  \033[36m%-36s\033[0m 构建 %s 二进制可执行文件。\n", s "-build", s; \
				printf "  \033[36m%-36s\033[0m 注入本地 Nacos 环境变量在宿主机运行 %s。\n", s "-run", s; \
				printf "  \033[36m%-36s\033[0m 运行 %s 全部单元测试。\n", s "-test", s; \
				printf "  \033[36m%-36s\033[0m 运行 %s 代码静态检查（golangci-lint）。\n", s "-lint", s; \
				printf "  \033[36m%-36s\033[0m 执行 %s 完整本地验证流程。\n", s "-verify", s; \
			} \
		}' $(MAKEFILE_LIST)

##@ 全局工作区与质量门禁

build: $(addsuffix -build,$(BUSINESS_SERVICES)) ## 构建全部已登记业务服务二进制。

test: platform-test $(addsuffix -test,$(BUSINESS_SERVICES)) tools-openapi-convert-test tools-nacos-config-seed-test tools-service-scaffold-test ## 运行全部 Go 模块单元测试。

test-containers: platform-test-containers $(addsuffix -test-containers,$(BUSINESS_SERVICES)) ## 运行 platform 与已登记业务服务的全部 Docker-backed 测试。

coverage: ## 生成测试覆盖率报告并校验关键包覆盖率门槛。
	./scripts/check-coverage.sh

lint: platform-lint $(addsuffix -lint,$(BUSINESS_SERVICES)) tools-openapi-convert-lint tools-nacos-config-seed-lint tools-service-scaffold-lint ## 运行全部 Go 模块的 golangci-lint。

verify: lint $(addsuffix -architecture-lint,$(BUSINESS_SERVICES)) github-actions-pins-check delivery-verify platform-generate $(addsuffix -generate,$(BUSINESS_SERVICES)) test $(addsuffix -openapi-generate,$(BUSINESS_SERVICES)) ## 运行完整本地交付验证（代码检查、测试、生成物一致性校验）。
	@if [ -e .git ]; then git diff --exit-code -- . ':(exclude)AGENTS.md' ':(exclude)openspec/AGENTS.md' ':(exclude)CLAUDE.md' ':(exclude).multica/project/resources.json' ':(exclude).multica/**'; \
	else echo "git diff: skipped (project has no .git)"; fi

##@ 跨服务共享平台 (Platform)

platform-test: ## 运行 platform 共享模块测试。
	$(MAKE) -C $(PLATFORM_DIR) test

platform-test-containers: ## 运行 platform 模块的 Docker-backed 容器集成测试。
	$(MAKE) -C $(PLATFORM_DIR) test-containers

platform-lint: ## 运行 platform 共享模块代码静态检查。
	$(MAKE) -C $(PLATFORM_DIR) lint

platform-generate: ## 生成 platform 模块的 Go 生成物（如 mock、代码生成等）。
	$(MAKE) -C $(PLATFORM_DIR) generate

platform-verify: ## 执行 platform 共享模块完整验证。
	$(MAKE) -C $(PLATFORM_DIR) verify

##@ 仓库级交付工具 (Tools)

new-service: ## 使用脚手架生成新业务服务骨架，用法：make new-service name=<service-name> [port=<port>]
	@test -n "$(name)" || (echo "用法：make new-service name=<service-name> [port=<port>]" >&2; exit 2)
	go run ./$(TOOLS_SERVICE_SCAFFOLD_DIR) -name '$(name)' $(if $(port),-port '$(port)',)

tools-service-scaffold-test: ## 运行业务服务脚手架生成工具单元测试。
	$(MAKE) -C $(TOOLS_SERVICE_SCAFFOLD_DIR) test

tools-service-scaffold-lint: ## 运行业务服务脚手架生成工具代码静态检查。
	$(MAKE) -C $(TOOLS_SERVICE_SCAFFOLD_DIR) lint

tools-openapi-convert-test: ## 运行 OpenAPI 转换工具单元测试。
	$(MAKE) -C $(TOOLS_OPENAPI_CONVERT_DIR) test

tools-openapi-convert-lint: ## 运行 OpenAPI 转换工具代码静态检查。
	$(MAKE) -C $(TOOLS_OPENAPI_CONVERT_DIR) lint

tools-nacos-config-seed-test: ## 运行 Nacos 配置初始化工具单元测试。
	$(MAKE) -C $(TOOLS_NACOS_CONFIG_SEED_DIR) test

tools-nacos-config-seed-lint: ## 运行 Nacos 配置初始化工具代码静态检查。
	$(MAKE) -C $(TOOLS_NACOS_CONFIG_SEED_DIR) lint

tools-nacos-config-seed-local-host: ## 向本地 Nacos 发布宿主机进程配置（支持 NACOS_ADDR/NACOS_GROUP/NACOS_TIMEOUT 覆盖）。
	go run ./$(TOOLS_NACOS_CONFIG_SEED_DIR) \
		-addr '$(NACOS_ADDR)' \
		-namespace loca-host \
		-group '$(NACOS_GROUP)' \
		-config-dir deployments/nacos/local-host \
		-timeout '$(NACOS_TIMEOUT)'

tools-nacos-config-seed-local-docker: ## 向本地 Nacos 发布 Compose 容器配置（支持 NACOS_ADDR/NACOS_GROUP/NACOS_TIMEOUT 覆盖）。
	go run ./$(TOOLS_NACOS_CONFIG_SEED_DIR) \
		-addr '$(NACOS_ADDR)' \
		-namespace loca-docker \
		-group '$(NACOS_GROUP)' \
		-config-dir deployments/nacos/local-docker \
		-timeout '$(NACOS_TIMEOUT)'

# ------------------------------------------------------------------------------
# 业务微服务通用生命周期规则（针对 BUSINESS_SERVICES 中注册的每个服务动态注入）
# ------------------------------------------------------------------------------

define BUSINESS_SERVICE_RULES
$(1)-build:
	@mkdir -p bin
	$$(MAKE) -C $(1) build

$(1)-run:
	$$(call LOCAL_NACOS_ENV,$(1)) $$(MAKE) -C $(1) run

$(1)-test:
	$$(MAKE) -C $(1) test

$(1)-test-containers:
	$$(MAKE) -C $(1) test-containers

$(1)-lint:
	$$(MAKE) -C $(1) lint

$(1)-generate:
	$$(MAKE) -C $(1) generate

$(1)-openapi-generate:
	$$(MAKE) -C $(1) openapi-generate

$(1)-architecture-lint:
	$$(MAKE) -C $(1) architecture-lint

$(1)-verify:
	$$(MAKE) -C $(1) verify

.PHONY: $(1)-build $(1)-run $(1)-test $(1)-test-containers $(1)-lint $(1)-generate $(1)-openapi-generate $(1)-architecture-lint $(1)-verify
endef

$(foreach svc,$(BUSINESS_SERVICES),$(eval $(call BUSINESS_SERVICE_RULES,$(svc))))

##@ 业务微服务通用生命周期
##%SERVICES_PLACEHOLDER%

##@ User Service 专属运维与架构指令

user-service-rbac-sync-race-test: ## 使用 race detector 运行 user-service 的 RBAC policy sync 并发竞争测试。
	$(MAKE) -C user-service rbac-sync-race-test

user-service-seed-rbac: ## 运行 RBAC 数据初始化，同步权限基线与默认角色绑定到数据库。
	$(call LOCAL_NACOS_ENV,user-service) $(MAKE) -C user-service seed-rbac

user-service-bootstrap-super-admin: ## 一次性创建初始超级管理员账号（需设置 ADMIN_USERNAME 与 ADMIN_BOOTSTRAP_PASSWORD）。
	$(call LOCAL_NACOS_ENV,user-service) $(MAKE) -C user-service bootstrap-super-admin ADMIN_USERNAME='$(ADMIN_USERNAME)' ADMIN_NICKNAME='$(ADMIN_NICKNAME)'

user-service-image-verify: ## 校验 user-service 生产 Distroless 镜像内容与安全性。
	./deployments/docker/verify-user-service-image.sh aegiscore-user-service:latest

user-service-migrate-diff: ## 对比 Ent schema 与数据库状态并生成新的 SQL 迁移脚本（需传 name=<migration-name>）。
	@test -n "$(name)" || (echo "用法：make user-service-migrate-diff name=<migration-name>" >&2; exit 2)
	$(MAKE) -C user-service migrate-diff name='$(name)'

user-service-migrate-validate: ## 校验 user-service 的 Atlas 数据库迁移历史与 checksum 一致性。
	$(MAKE) -C user-service migrate-validate

user-service-fxgraph-generate: ## 生成 user-service 的 Fx 依赖注入图 DOT 文件。
	$(MAKE) -C user-service fxgraph-generate

user-service-fxgraph-check: ## 检查 user-service 的 Fx 依赖注入图是否存在未提交的变更漂移。
	$(MAKE) -C user-service fxgraph-check

##@ 部署与交付验证 (Deployments & Delivery)

compose-dashboard-generate: ## 从通用观测仪表盘模板生成 Compose Grafana 仪表盘定义。
	./deployments/compose/scripts/generate-grafana-dashboard.sh

compose-dashboard-check: ## 检查 Compose Grafana 仪表盘是否存在未同步的漂移。
	./deployments/compose/scripts/generate-grafana-dashboard.sh --check

local-init: ## 本地联调一键初始化：执行 Nacos 鉴权检查、双环境配置推送并初始化 RBAC 基线数据。
	docker compose -f deployments/compose/docker-compose.yml --profile init run --rm nacos-init-auth
	docker compose -f deployments/compose/docker-compose.yml --profile init run --rm nacos-init-host
	docker compose -f deployments/compose/docker-compose.yml --profile init run --rm nacos-init-docker
	docker compose -f deployments/compose/docker-compose.yml restart user-service
	docker compose -f deployments/compose/docker-compose.yml --profile init run --rm rbac-seed

delivery-verify: ## 校验 Helm Chart、Kustomize、Compose、Prometheus 规则和 Dashboard 部署资产一致性。
	./scripts/verify-delivery-assets.sh

github-actions-pins-check: ## 检查 GitHub Actions 中第三方 Action 是否强制锁定了安全的 commit SHA。
	./scripts/check-github-actions-pins.sh
