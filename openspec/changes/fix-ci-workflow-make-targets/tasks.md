## 1. 工作流修复

- [x] 1.1 将 `.github/workflows/ci.yml` verify job 中的 `make architecture-lint` 替换为 `make user-service-architecture-lint`。
- [x] 1.2 将 `.github/workflows/ci.yml` verify job 中的 `make openapi-generate` 替换为 `make user-service-openapi-generate`，并保留 `git diff --exit-code`。
- [x] 1.3 将 `.github/workflows/migrations.yml` 中的 `make migrate-validate` 替换为 `make user-service-migrate-validate`。

## 2. 验证与收尾

- [x] 2.1 搜索 `.github/workflows` 中的 `architecture-lint`、`openapi-generate` 和 `migrate-validate` 调用，确认不再存在无服务上下文的错误根目标。
- [x] 2.2 运行 `make -n user-service-architecture-lint user-service-openapi-generate user-service-migrate-validate`，确认根目标存在并展开到 user-service 对应脚本。
- [x] 2.3 运行 `make user-service-architecture-lint`，确认 OpenSpec change artifacts 与 OPSX 文档约束通过。
- [x] 2.4 检查 `git diff --exit-code -- openspec/changes/fix-ci-workflow-make-targets .github/workflows/ci.yml .github/workflows/migrations.yml` 的结果，确认只有预期变更。
