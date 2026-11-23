# ==============================================================================
# AegisCore Platform 共享契约与底层原语 Makefile
# ==============================================================================

CONTAINER_TEST_PACKAGES := ./testing/containers

.PHONY: help test test-containers lint generate verify

help: ## 查看 platform 模块可用命令与分类指引。
	@awk 'BEGIN {FS = ":.*##"; printf "\nplatform 可用指令清单：\n  make \033[36m<目标>\033[0m\n"} \
		/^##@/ { printf "\n\033[1;33m%s\033[0m\n", substr($$0, 5) } \
		/^[a-zA-Z0-9_-]+:.*##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

##@ 质量检查与测试

test: ## 运行 platform 共享原语模块全部单元测试。
	go test ./...

test-containers: ## 运行平台层 Docker-backed 容器集成测试（如 Redis/Postgres 真实连接测试）。
	go test -v -count=1 $(CONTAINER_TEST_PACKAGES) -args -aegiscore.testcontainers

lint: ## 运行 platform 共享模块的 golangci-lint 静态代码检查。
	golangci-lint run ./...

##@ 代码生成与交付验证

generate: ## 执行 platform 模块的 go generate 代码生成。
	go generate ./...

verify: lint generate test ## 运行 platform 模块完整验证（静态检查、代码生成和单元测试）。
	@if [ -e ../.git ]; then git diff --exit-code -- .; \
	else echo "git diff: skipped (project has no .git)"; fi
