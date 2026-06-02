## ADDED Requirements

### Requirement: Migrate user token version schema
系统 SHALL 通过 Ent schema 和 Atlas SQL migration 表达用户表 `token_version` 字段变更。迁移文件 MUST 写入 `user-services/migrations/`，并且 MUST 与 `atlas.sum` 一起维护；用户服务运行时 MUST NOT 通过 `client.Schema.Create(ctx)` 自动创建或修改该字段。

#### Scenario: Generate migration after adding token version field
- **Given** `user-services/ent/schema/user.go` 新增 `token_version` 字段且默认值为 `1`
- **When** 开发者运行用户服务迁移生成命令
- **Then** Atlas MUST 在 `user-services/migrations/` 生成新增 `token_version` 字段的 SQL migration
- **Then** 生成流程 MUST 更新或校验 `user-services/migrations/atlas.sum`

#### Scenario: Runtime does not create token version schema implicitly
- **Given** `token_version` schema 变更已经通过 SQL migration 表达
- **When** 用户服务 HTTP runtime 启动
- **Then** 服务 MUST NOT 通过 `client.Schema.Create(ctx)` 自动创建或修改 `token_version` 字段
- **Then** 数据库 schema 变更 MUST 通过已生成并部署的 SQL migration 文件完成
