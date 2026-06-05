## Why

用户状态默认值同时存在于 Ent schema 持久化契约和领域枚举中，当前两处都使用 `100`，但缺少自动化一致性保护。若未来只修改 `domain.UserStatusNormal` 或 schema 默认值其中一处，数据库默认用户状态可能与领域正常状态语义漂移。

## What Changes

- 为用户服务新增一致性测试，验证 Ent `User` schema 的 `status` 默认值与领域枚举 `domain.UserStatusNormal` 保持一致。
- 在 Ent schema 默认值注释中明确该持久化默认值必须与 `domain.UserStatusNormal` 保持一致。
- 不修改数据库字段类型、默认值数字、索引、约束、API 响应或运行时配置。
- 不生成新的 Atlas SQL migration，也不修改既有 migration 历史。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `database-schema-migrations`: 补充对用户状态默认值来源与领域枚举一致性的测试约束，确保 schema 默认值语义调整不会静默漂移。

## Impact

- 影响代码：`user-services/ent/schema/user/schema.go`、`user-services/internal/domain/user_status.go` 及新增或更新的用户服务测试文件。
- API 兼容性：无外部 HTTP API、错误码或响应结构变化。
- 数据模型兼容性：`users.status` 默认值继续为 `100`，不产生数据库结构变更。
- 依赖和配置：不新增运行时依赖，不修改配置项。
