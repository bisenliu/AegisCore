## ADDED Requirements

### Requirement: Migrate user identity fields
系统 SHALL 通过 Ent schema 和 Atlas SQL migration 表达用户表移除 `email`、新增 `username`、新增 `user_id` 以及相关唯一约束变更。迁移文件 MUST 写入 `user-services/migrations/`，并且 MUST 与 `atlas.sum` 一起维护；用户服务运行时 MUST NOT 通过 `client.Schema.Create(ctx)` 自动创建或修改这些字段。

#### Scenario: Generate migration after user identity schema change
- **Given** `user-services/ent/schema/user.go` 删除 `email` 字段并新增 `username` 与 `user_id` 字段
- **When** 开发者运行用户服务迁移生成命令
- **Then** Atlas MUST 在 `user-services/migrations/` 生成用户身份字段变更 SQL migration
- **Then** migration MUST 表达 `username` 非空和唯一约束
- **Then** migration MUST 表达 `user_id` 非空、唯一和不可变业务语义对应的数据库约束
- **Then** migration MUST 删除 `email` 列及其唯一约束
- **Then** 生成流程 MUST 更新或校验 `user-services/migrations/atlas.sum`

#### Scenario: Review migration data backfill
- **Given** 目标数据库中可能存在历史用户记录
- **When** 开发者审查用户身份字段 migration
- **Then** migration MUST 明确 `user_id` 回填策略
- **Then** migration MUST 明确 `username` 回填策略或要求部署前人工准备数据
- **Then** migration MUST NOT 静默生成重复 `username` 或空 `user_id`

#### Scenario: Runtime does not create identity schema implicitly
- **Given** 用户身份 schema 变更已经通过 SQL migration 表达
- **When** 用户服务 HTTP runtime 启动
- **Then** 服务 MUST NOT 通过 `client.Schema.Create(ctx)` 自动创建、删除或修改 `email`、`username` 或 `user_id` 字段
- **Then** 数据库 schema 变更 MUST 通过已生成并部署的 SQL migration 文件完成
