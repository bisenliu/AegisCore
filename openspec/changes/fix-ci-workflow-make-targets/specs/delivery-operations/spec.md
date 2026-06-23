## ADDED Requirements

### Requirement: CI 工作流使用存在的服务前缀根目标

系统 MUST 确保 GitHub Actions 中的交付校验步骤调用仓库根 `Makefile` 中存在的目标；当步骤执行 user-service 私有交付能力时，目标名称 MUST 使用 `user-service-` 前缀。

#### Scenario: PR 门禁运行 user-service 架构 lint

- **WHEN** GitHub Actions PR 或 push verify job 需要执行 user-service 架构边界检查
- **THEN** 工作流 MUST 调用 `make user-service-architecture-lint`

#### Scenario: PR 门禁运行 user-service OpenAPI 生成

- **WHEN** GitHub Actions PR 或 push verify job 需要检查 user-service OpenAPI 生成物是否存在 drift
- **THEN** 工作流 MUST 调用 `make user-service-openapi-generate`
- **THEN** 工作流 MUST 继续通过 `git diff --exit-code` 暴露生成物 drift

#### Scenario: migration 工作流校验 user-service migrations

- **WHEN** GitHub Actions migration validation job 需要校验 user-service Atlas migrations
- **THEN** 工作流 MUST 调用 `make user-service-migrate-validate`

#### Scenario: 禁止无服务上下文目标

- **WHEN** GitHub Actions workflow 需要调用 user-service 私有 lint、生成或 migration 目标
- **THEN** 工作流 MUST NOT 调用根 `Makefile` 中不存在的 `architecture-lint`、`openapi-generate` 或 `migrate-validate` 目标
