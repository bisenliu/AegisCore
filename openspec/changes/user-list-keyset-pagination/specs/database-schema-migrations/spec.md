## ADDED Requirements

### Requirement: Maintain user indexes for keyset list queries
系统 SHALL 通过 Ent schema 和 Atlas SQL migration 表达用户列表 keyset pagination 所需索引。用户列表默认按未软删除记录和 `user_id ASC` 前进，且支持 status 过滤时，索引策略 MUST 覆盖对应查询路径。迁移文件 MUST 写入 `user-services/migrations/`，并且 MUST 与 `atlas.sum` 一起维护；用户服务运行时 MUST NOT 通过 `client.Schema.Create(ctx)` 自动创建或修改这些索引。

#### Scenario: Add deleted at user id keyset index
- **Given** `GET /api/v1/users` 默认只查询未软删除用户并按 `user_id ASC` 排序
- **When** 开发者修改 `user-services/ent/schema/` 下的 Ent `User` schema
- **Then** Ent schema MUST 声明 `deleted_at`、`user_id` 组合索引
- **Then** Atlas MUST 在 `user-services/migrations/` 生成对应索引 SQL migration
- **Then** 生成流程 MUST 更新或校验 `user-services/migrations/atlas.sum`

#### Scenario: Add status deleted at user id keyset index
- **Given** 用户列表支持高频 `status` 过滤并按 `user_id ASC` 排序
- **When** 开发者修改 `User` Ent schema 的索引定义
- **Then** Ent schema MUST 声明 `status`、`deleted_at`、`user_id` 组合索引
- **Then** Atlas migration MUST 表达该索引用于 status 过滤下的 keyset 查询路径
- **Then** migration SQL MUST 可被人工审查

#### Scenario: Runtime does not create keyset indexes implicitly
- **Given** keyset pagination 索引变更已经通过 SQL migration 表达
- **When** 用户服务 HTTP runtime 启动
- **Then** 服务 MUST NOT 通过 `client.Schema.Create(ctx)` 自动创建或修改索引
- **Then** 数据库 schema 变更 MUST 通过已生成并部署的 SQL migration 文件完成
