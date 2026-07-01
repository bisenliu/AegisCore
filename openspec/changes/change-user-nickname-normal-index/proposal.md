## Why

当前 `users.nickname` 使用 PostgreSQL GIN trigram 索引依赖 Ent 生成以外的插件相关配置，迁移生成和维护成本较高。需要将昵称索引恢复为 Ent 可稳定生成和审查的普通索引，并同步整理迁移文件，避免旧迁移和插件相关 SQL 继续漂移。

## What Changes

- 将 `user-service/ent/schema/user.go` 中 `nickname` 索引改为普通 Ent 字段索引，不再配置 `GIN`、`gin_trgm_ops` 或自定义 `users_nickname_trgm` 存储名。
- 删除现有旧 SQL migration 文件，只保留一个最新的完整迁移 SQL 文件，内容反映当前 schema 的最终结构。
- 最新迁移文件中的 `users.nickname` 仅保留普通 B-tree 索引，不再创建 PostgreSQL trigram GIN 索引。
- 保持用户创建、查询、列表、用户名唯一性、状态约束、软删除过滤和 HTTP API 语义不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-identity-management`: 调整用户昵称查询索引的持久化要求，从适合 contains 查询的 PostgreSQL trigram 策略改为普通 Ent 索引，并明确该调整不改变用户资料业务语义。

## Impact

- 影响 `user-service/ent/schema/user.go` 的 `User.Indexes` 定义和相关 import。
- 影响 `user-service/migrations/*.sql`，需要删除旧迁移并保留一个最新迁移文件。
- 不新增或修改 HTTP API、OpenAPI 响应契约、认证、RBAC 或共享模块接口。
- 数据库层面会移除昵称 trigram GIN 索引策略；昵称 contains 查询不再依赖 PostgreSQL 插件相关索引作为性能支撑。
