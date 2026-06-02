## ADDED Requirements

### Requirement: Preserve user schema field comments in migrations
系统必须通过 Ent schema 声明 `users` 表字段注释，并通过 Atlas migration 将字段 comment 写入用户服务迁移目录。

#### Scenario: Generate migration with user field comments
- **Given** `user-services/ent/schema/user.go` 为每个 `User` 字段声明了 comment
- **When** 开发者运行用户服务迁移生成命令
- **Then** Atlas 必须在 `user-services/migrations/` 生成包含字段 comment 变更的 SQL migration
- **Then** 生成流程必须更新或校验 `user-services/migrations/atlas.sum`

### Requirement: Migrate user password and millisecond timestamps
系统必须通过 Atlas SQL migration 表达用户表新增必填 `password` 字段以及 `created_at`、`updated_at` 毫秒时间戳字段变更，不得依赖服务运行时自动建表或改表。

#### Scenario: Generate migration after user timestamp and password schema change
- **Given** Ent `User` schema 新增非空 `password` 字段，并将 `created_at`、`updated_at` 定义为毫秒级 Unix 时间戳字段
- **When** 开发者运行迁移生成命令
- **Then** Atlas 必须生成用户表新增 `password` 字段的 SQL migration
- **Then** Atlas 必须生成 `created_at`、`updated_at` 类型或语义变更的 SQL migration
- **Then** 迁移 SQL 必须可被人工审查，并在修改后重新计算 `atlas.sum`

#### Scenario: Runtime does not apply schema changes implicitly
- **Given** 用户 schema 字段变更已经通过 SQL migration 表达
- **When** 用户服务 HTTP runtime 启动
- **Then** 服务不得通过 `client.Schema.Create(ctx)` 自动创建或修改数据库 schema
- **Then** 数据库 schema 变更必须通过已生成并部署的 SQL migration 文件完成
