## Why

`user-service/internal/persistence/ent/schema` 中多个 Ent schema 重复声明毫秒时间戳字段和字符串枚举校验逻辑，长期会增加默认值、注释和更新策略漂移的维护成本。

本次变更通过 schema 包内部共享 helper 收敛这些重复实现，并用 Ent 生成和 Atlas migration 校验确认不会意外改变数据库结构或交付语义。

## What Changes

- 在 Ent schema 包内提取通用毫秒时间戳 mixin，例如 `createdAtMillisMixin` 和 `updatedAtMillisMixin`。
- 将 `oneOfStrings` 从 `rbacpolicyoutboxevent.go` 移到 schema 包内部共享 helper，供后续 schema 复用。
- 统一 `created_at`、`updated_at` 字段的中文注释、`time.Now().UnixMilli()` 默认值和 `UpdateDefault` 策略。
- 更新受影响 Ent schema 以使用共享 helper，并重新生成 Ent 代码。
- 运行 Atlas migration diff/validate，确认本次重构不引入非预期 SQL、hash 或数据库结构漂移。
- 不改变 HTTP API、业务行为、列名、列类型、索引、默认时间语义或 RBAC outbox 投递语义。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: 明确 Ent schema 生成和 migration 校验覆盖 schema 内部 helper/mixin 重构场景，确保生成物和数据库结构来源保持一致。

## Impact

- 受影响代码：`user-service/internal/persistence/ent/schema/*.go`。
- 受影响生成物：`user-service/internal/persistence/ent/` 下 Ent 生成代码可能因 mixin 字段来源变化产生差异。
- 受影响验证：需要执行 `make user-service-generate`、`make user-service-migrate-diff name=standardize-ent-schema-helpers` 和 `make user-service-migrate-validate`。
- 数据库影响：预期不改变表、列、索引、注释、默认值或 migration SQL；如 Atlas diff 生成 SQL，必须审查并确认是否为预期。
- 能力影响：涉及 user 和 RBAC 相关 schema 的实现结构，但不修改 `user-identity-management` 或 `rbac-access-control` 的稳定业务需求。
