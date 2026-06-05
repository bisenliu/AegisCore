## Why

当前用户服务 Ent schema 通过导入 `user-services/internal/domain` 获取用户状态默认值，使数据库 schema 生成链路依赖业务领域层。该依赖会扩大 Ent/Atlas 生成链路的编译面，并在领域层后续引入更多业务依赖时污染持久化 schema 边界。

## What Changes

- 将 `user-services/ent/schema/user/schema.go` 的数据库默认值声明与 `internal/domain` 解耦，schema 层保留明确、稳定的数据库默认值。
- 保持 repository 继续负责 Ent 模型与 domain 类型转换，业务状态规则仍由 `internal/domain` 承载。
- 保持用户表 `status` 字段默认值、字段类型、注释、索引和 Atlas migration 历史不变。
- 不修改 HTTP API、响应信封、错误码、配置 key 或运行时依赖名称。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-domain-boundary`: 明确 Ent schema 不得导入业务 domain 作为数据库默认值来源，领域状态规则仍由 domain 和 repository 映射边界承载。
- `database-schema-migrations`: 明确仅调整默认值来源时必须保持数据库 schema 语义不变，不应生成新的 SQL migration。

## Impact

- 受影响代码：`user-services/ent/schema/user/schema.go`，以及必要时的 Ent 生成输出校验。
- 受影响能力：与 `docs/opsx/CAPABILITY_MAP.md` 对应的 `database-schema-migrations`；同时约束现有 `user-domain-boundary` 规格。
- API 兼容性：无外部 HTTP 行为变化，用户查询、创建、认证相关响应字段和错误码保持不变。
- 数据模型兼容性：用户表 `status` 数据库默认值继续为 `100`，不产生数据库结构变更或 migration 历史变更。
- 依赖影响：Ent schema 编译不再依赖 `internal/domain` 包，降低 schema/codegen 对业务层的耦合。
