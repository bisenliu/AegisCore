# ==============================================================================
# AegisCore Nacos 配置初始化工具 Makefile
# ==============================================================================

.PHONY: help test lint

help: ## 查看 Nacos 配置初始化工具可用命令。
	@awk 'BEGIN {FS = ":.*##"; printf "\nnacos-config-seed 可用指令清单：\n  make \033[36m<目标>\033[0m\n\n"} \
		/^[a-zA-Z0-9_-]+:.*##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

test: ## 运行 Nacos 配置初始化工具全部单元测试。
	go test ./...

lint: ## 运行 Nacos 配置初始化工具静态代码检查（golangci-lint）。
	golangci-lint run ./...
