# ==============================================================================
# AegisCore User Service 用户微服务 Makefile
# ==============================================================================

# 构建与运行变量定义
USER_SERVICE_BIN ?= ../bin/user-service
ADMIN_USERNAME ?=
ADMIN_NICKNAME ?=
export ADMIN_BOOTSTRAP_PASSWORD

# 本地默认 Nacos 环境变量注入模板（调用方已导出的环境变量优先）
LOCAL_RUN_ENV = \
	AEGISCORE_SERVICE=$${AEGISCORE_SERVICE:-user-service} \
	AEGISCORE_NACOS_ADDR=$${AEGISCORE_NACOS_ADDR:-127.0.0.1:8848} \
	AEGISCORE_NACOS_NAMESPACE=$${AEGISCORE_NACOS_NAMESPACE:-loca-host} \
	AEGISCORE_NACOS_GROUP=$${AEGISCORE_NACOS_GROUP:-AEGISCORE} \
	AEGISCORE_NACOS_USERNAME=$${AEGISCORE_NACOS_USERNAME:-nacos} \
	AEGISCORE_NACOS_PASSWORD=$${AEGISCORE_NACOS_PASSWORD:-nacos}

# 测试目录包集合
CONTAINER_TEST_PACKAGES := ./internal/features/permission/infrastructure/postgres ./internal/features/role/infrastructure/postgres ./tests/e2e
RBAC_SYNC_RACE_PACKAGES := ./internal/features/permission/infrastructure/redis ./internal/features/permission/infrastructure/casbin

# 声明伪目标
.PHONY: help build run test test-containers rbac-sync-race-test lint architecture-lint architecture-lint-test verify
.PHONY: seed-rbac bootstrap-super-admin generate migrate-diff migrate-validate openapi-generate fxgraph-generate fxgraph-check

# ------------------------------------------------------------------------------
# 帮助信息
# ------------------------------------------------------------------------------

help: ## 查看 user-service 可用命令与分类指引。
	@awk 'BEGIN {FS = ":.*##"; printf "\nuser-service 可用指令清单：\n  make \033[36m<目标>\033[0m\n"} \
		/^##@/ { printf "\n\033[1;33m%s\033[0m\n", substr($$0, 5) } \
		/^[a-zA-Z0-9_-]+:.*##/ { printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

##@ 编译构建与本地运行

build: ## 构建 user-service 二进制可执行文件。
	@mkdir -p $(dir $(USER_SERVICE_BIN))
	go build -o $(USER_SERVICE_BIN) ./cmd

run: ## 运行 user-service 服务（默认注入本地 Nacos 环境，已导出的环境变量优先）。
	$(LOCAL_RUN_ENV) go run ./cmd serve

##@ 质量检查与架构边界

test: ## 运行 user-service 全部单元测试。
	go test ./...

test-containers: ## 运行基于 Testcontainers 的真实 PostgreSQL/Redis 容器集成测试。
	go test -v -count=1 $(CONTAINER_TEST_PACKAGES) -args -aegiscore.testcontainers

rbac-sync-race-test: ## 使用 race detector 针对 RBAC 策略同步进行高并发竞争检测测试。
	go test -race $(RBAC_SYNC_RACE_PACKAGES)

lint: ## 运行 user-service 代码静态检查（golangci-lint）。
	golangci-lint run ./...

architecture-lint: architecture-lint-test ## 执行 user-service 严格架构边界、分层依赖与代码防漂移校验。
	./scripts/architecture/lint.sh

architecture-lint-test: ## 使用故意制造的违规 fixture 验证架构检查脚本是否能准确捕获分层污染。
	./scripts/architecture/lint-test.sh

verify: lint architecture-lint generate test openapi-generate ## 执行 user-service 完整流水线验证。
	@if [ -e ../.git ]; then git diff --exit-code; \
	else echo "git diff: skipped (project has no .git)"; fi

##@ 数据库 Schema 与迁移

migrate-diff: ## 对比 Ent schema 与 Atlas 状态生成新的 SQL 迁移脚本（用法：make migrate-diff name=<name>）。
	@test -n "$(name)" || (echo "用法：make migrate-diff name=<migration-name>" >&2; exit 2)
	./scripts/migrate-diff.sh "$(name)"

migrate-validate: ## 使用 Atlas 校验 SQL 迁移文件语法与 atlas.sum 校验和一致性。
	atlas migrate validate --dir file://migrations

##@ 代码与架构契约生成

generate: ## 执行 Ent ORM 代码生成。
	go generate ./...

openapi-generate: ## 解析接口注解并生成 OpenAPI 3 文档与 Go 资源嵌入。
	./scripts/openapi-generate.sh

fxgraph-generate: ## 生成 user-service 的 Uber Fx 依赖图（DOT 格式）。
	go run ./cmd fxgraph --output ./docs/fx-dependency-graph.dot

fxgraph-check: fxgraph-generate ## 检查 user-service 的 Fx 依赖注入图是否存在未提交的变更漂移。
	@if [ -e ../.git ]; then git diff --exit-code -- docs/fx-dependency-graph.dot; \
	else echo "git diff: skipped (project has no .git)"; fi

##@ RBAC 权限初始化与运维引导

seed-rbac: ## 执行 RBAC 基础权限基线与默认角色绑定的数据库初始化。
	$(LOCAL_RUN_ENV) go run ./cmd rbac seed

bootstrap-super-admin: ## 一次性创建系统初始超级管理员（需指定 ADMIN_USERNAME 与 ADMIN_BOOTSTRAP_PASSWORD）。
	@test -n "$(ADMIN_USERNAME)" || (echo "必须设置 ADMIN_USERNAME" >&2; exit 2)
	@test -n "$${ADMIN_BOOTSTRAP_PASSWORD}" || (echo "必须设置 ADMIN_BOOTSTRAP_PASSWORD" >&2; exit 2)
	$(LOCAL_RUN_ENV) go run ./cmd rbac bootstrap-super-admin --username '$(ADMIN_USERNAME)' --nickname '$(ADMIN_NICKNAME)' --password-env ADMIN_BOOTSTRAP_PASSWORD
