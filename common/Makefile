.PHONY: help test lint generate verify

help: ## 查看 common 可用命令。
	@awk 'BEGIN {FS = ":.*##"; printf "common 可用命令：\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## 运行 common 测试。
	go test ./...

lint: ## 运行 common lint。
	golangci-lint run ./...

generate: ## 生成 common Go 生成物。
	go generate ./...

verify: lint generate test ## 运行 common 验证。
	git diff --exit-code -- .
