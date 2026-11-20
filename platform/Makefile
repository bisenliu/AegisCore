CONTAINER_TEST_PACKAGES := ./testing/containers

.PHONY: help test test-containers lint generate verify

help: ## 查看 platform 可用命令。
	@awk 'BEGIN {FS = ":.*##"; printf "platform 可用命令：\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## 运行 platform 测试。
	go test ./...

test-containers: ## 运行 platform Docker-backed 测试。
	go test -v -count=1 $(CONTAINER_TEST_PACKAGES) -args -aegiscore.testcontainers

lint: ## 运行 platform lint。
	golangci-lint run ./...

generate: ## 生成 platform Go 生成物。
	go generate ./...

verify: lint generate test ## 运行 platform 验证。
	@if [ -e ../.git ]; then git diff --exit-code -- .; \
	else echo "git diff: skipped (project has no .git)"; fi
