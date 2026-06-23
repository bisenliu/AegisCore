## Why

当前 GitHub Actions 工作流调用了根 `Makefile` 中不存在的 `architecture-lint`、`openapi-generate` 和 `migrate-validate` 目标，导致 PR 门禁和 migration 发布校验在执行阶段失败。仓库已经定义了带服务上下文的根目标，本变更需要让 CI 与现有 Make 入口保持一致，恢复交付流水线的可靠性。

## What Changes

- 将 `.github/workflows/ci.yml` 的 verify job 调整为调用 `make user-service-architecture-lint` 和 `make user-service-openapi-generate`。
- 将 `.github/workflows/migrations.yml` 的 migration validate step 调整为调用 `make user-service-migrate-validate`。
- 保持现有 root `Makefile` 目标命名规则，不新增无服务上下文的根目标。
- 保持 CI 现有 lint、test、OpenAPI drift 检查和 migration validate 行为不变，只修正入口目标名称。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: CI 与 migration 工作流必须调用仓库根 `Makefile` 中存在且带服务上下文的交付目标，避免 PR 门禁和发布校验因目标缺失而失败。

## Impact

- 影响 `.github/workflows/ci.yml` 和 `.github/workflows/migrations.yml`。
- 不改变 HTTP API、OpenAPI 内容、数据库 schema、migration SQL、运行时代码或部署资产。
- 不引入新依赖。
- 修复后 PR 与 push 工作流将继续执行既有验证链路，并通过已存在的 user-service 前缀目标运行架构 lint、OpenAPI 生成和 migration 校验。
