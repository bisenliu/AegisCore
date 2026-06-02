## Why

当前用户 Ent schema 缺少密码字段，无法支撑用户创建时持久化必填密码数据；同时 `created_at`、`updated_at` 使用 `time.Time`，与需要以毫秒时间戳存储和输出的约定不一致。

本变更将用户表核心字段约束、时间字段类型和字段注释一次性纳入 schema 与 migration 工作流，避免后续用户创建和查询能力继续依赖不完整的数据模型。

## What Changes

- 在 `user-services/ent/schema/user.go` 的 `User` schema 中新增必填 `password` 字段。
- 将 `created_at`、`updated_at` 从时间类型调整为毫秒级 Unix 时间戳字段。
- 为 `User` schema 的每个字段增加数据库字段注释。
- 重新生成 Ent 代码，并通过 Atlas 生成用户服务数据库 migration。
- **BREAKING**: 用户表新增非空 `password` 字段，并改变 `created_at`、`updated_at` 的数据库类型/语义，现有数据迁移需要提供安全的默认或回填策略。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-profile-query`: 用户资料响应和 schema 约束中的 `created_at`、`updated_at` 必须改为毫秒时间戳语义；查询响应不得暴露 `password`。
- `user-profile-create`: 创建用户时必须持久化必填 `password`，并返回不包含 `password` 的用户资料响应；创建后的时间字段必须为毫秒时间戳。
- `database-schema-migrations`: 用户 Ent schema 结构变更必须通过 Ent 生成代码和 Atlas SQL migration 表达，并包含字段注释变更。

## Impact

- 影响代码：`user-services/ent/schema/user.go`、Ent 生成代码、用户 repository/service/controller DTO 映射、相关测试。
- 影响数据库：`users` 表新增非空 `password` 字段，`created_at` 和 `updated_at` 改为毫秒时间戳字段，并为所有用户字段添加 comment。
- 影响 API：用户创建请求需要包含 `password`；查询和创建响应继续返回用户资料但不得包含密码字段，时间字段以毫秒时间戳表示。
- 影响迁移：需要生成并校验 `user-services/migrations/` 下的新 Atlas migration 与 `atlas.sum`。
