## MODIFIED Requirements

### Requirement: Generate SQL migrations from Ent schema with Atlas
系统必须使用 Atlas 对比 Ent schema 与目标数据库状态生成 `.sql` migration 文件。迁移生成不得依赖服务运行时调用 `client.Schema.Create(ctx)`。

#### Scenario: Generate migration after Ent user lifecycle schema change
- **Given** 开发者修改 `user-services/ent/schema/` 下的 Ent `User` schema
- **When** 开发者运行迁移生成命令
- **Then** Atlas 必须通过 Ent schema source 读取期望数据库结构
- **Then** Atlas 必须在 `user-services/migrations/` 生成新的 SQL migration 文件
- **Then** migration 必须表达删除 `active`、新增 `status`、重命名或迁移 `name` 到 `nickname`、重命名或迁移 `password` 到 `password_hash`、新增 `deleted_at`
- **Then** 生成流程必须更新或校验 `user-services/migrations/atlas.sum`

### Requirement: Preserve user schema field comments in migrations
系统必须通过 Ent schema 声明 `users` 表字段注释，并通过 Atlas migration 将字段 comment 写入用户服务迁移目录。

#### Scenario: Generate migration with updated user field comments
- **Given** `user-services/ent/schema/user.go` 为每个 `User` 字段声明了 comment
- **When** 开发者运行用户服务迁移生成命令
- **Then** Atlas 必须在 `user-services/migrations/` 生成包含字段 comment 变更的 SQL migration
- **Then** 字段 comment 必须使用 `nickname`、`password_hash`、`status` 和 `deleted_at` 的新语义
- **Then** 生成流程必须更新或校验 `user-services/migrations/atlas.sum`

### Requirement: Migrate user password and millisecond timestamps
系统必须通过 Atlas SQL migration 表达用户表必填 `password_hash` 字段以及 `created_at`、`updated_at` 毫秒时间戳字段变更，不得依赖服务运行时自动建表或改表。

#### Scenario: Generate migration after user timestamp and password hash schema change
- **Given** Ent `User` schema 将密码持久化字段定义为非空 `password_hash`，并将 `created_at`、`updated_at` 定义为毫秒级 Unix 时间戳字段
- **When** 开发者运行迁移生成命令
- **Then** Atlas 必须生成用户表新增、重命名或迁移 `password_hash` 字段的 SQL migration
- **Then** Atlas 必须生成 `created_at`、`updated_at` 类型或语义变更的 SQL migration
- **Then** 迁移 SQL 必须可被人工审查，并在修改后重新计算 `atlas.sum`

#### Scenario: Runtime does not apply schema changes implicitly
- **Given** 用户 schema 字段变更已经通过 SQL migration 表达
- **When** 用户服务 HTTP runtime 启动
- **Then** 服务不得通过 `client.Schema.Create(ctx)` 自动创建或修改数据库 schema
- **Then** 数据库 schema 变更必须通过已生成并部署的 SQL migration 文件完成

#### Scenario: Runtime schema create is not used for migrations
- **Given** 数据库 schema 变更已经通过 SQL migration 表达
- **When** 用户服务启动 HTTP runtime
- **Then** 服务不得通过 `client.Schema.Create(ctx)` 自动创建或修改数据库 schema
- **Then** 数据库 schema 变更必须通过已生成并部署的 SQL migration 文件完成

## ADDED Requirements

### Requirement: Migrate user status nickname and soft delete schema
系统 MUST 通过 Ent schema 和 Atlas SQL migration 表达用户表字段收敛：`active` 删除并迁移为 `status`，`name` 重命名或迁移为 `nickname`，`password` 重命名或迁移为 `password_hash`，新增 nullable `deleted_at` 表示软删除时间。

#### Scenario: Migrate active values into status
- **Given** 既有用户表包含 `active` 布尔字段
- **When** migration 应用于目标数据库
- **Then** `active=true` 的既有记录 MUST 迁移为 `status=100`
- **Then** `active=false` 的既有记录 MUST 迁移为 `status=200`
- **Then** `status` 字段 MUST 非空并具备默认值 `100`
- **Then** migration 完成后长期 schema MUST 不再包含 `active`

#### Scenario: Rename name to nickname without data loss
- **Given** 既有用户表包含 `name` 字段
- **When** migration 应用于目标数据库
- **Then** 既有 `name` 值 MUST 保留到 `nickname`
- **Then** `nickname` MUST 保持非空且最大长度为 128
- **Then** migration 完成后长期 schema MUST 不再包含 `name`

#### Scenario: Rename password to password hash without data loss
- **Given** 既有用户表包含 `password` 字段且存储密码哈希
- **When** migration 应用于目标数据库
- **Then** 既有 `password` 值 MUST 保留到 `password_hash`
- **Then** `password_hash` MUST 保持非空
- **Then** migration 完成后长期 schema MUST 不再包含持久化字段 `password`

#### Scenario: Add deleted at for soft delete
- **Given** 用户表 schema 正在迁移
- **When** migration 应用于目标数据库
- **Then** 用户表 MUST 新增 nullable `deleted_at` 字段
- **Then** 既有记录的 `deleted_at` MUST 为 `NULL`
- **Then** `deleted_at=NULL` MUST 表示未删除

### Requirement: Maintain user indexes for status nickname and soft delete queries
系统 MUST 审查并更新用户表索引，使常用查询条件 `email`、`nickname`、`status` 和 `deleted_at IS NULL` 能与新的字段命名和软删除语义一致。

#### Scenario: Update indexes after field rename
- **Given** 用户表索引引用旧字段 `name` 或 `active`
- **When** migration 生成或人工审查 SQL
- **Then** 新索引 MUST 引用 `nickname` 或 `status`
- **Then** migration 完成后索引定义 MUST NOT 引用 `name` 或 `active`

#### Scenario: Preserve or document email uniqueness with soft delete
- **Given** 用户表需要按邮箱识别用户
- **When** migration 审查邮箱唯一索引
- **Then** 实现 MUST 明确选择全表唯一邮箱索引或 `deleted_at IS NULL` partial unique index
- **Then** repository 的用户存在性检查 MUST 与该索引语义一致
- **Then** migration SQL 和实现说明 MUST 记录所选唯一性策略

#### Scenario: Index active user lookup paths
- **Given** 查询、列表和登录默认只访问未软删除用户
- **When** migration 审查索引
- **Then** 系统 MUST 为 `deleted_at` 相关过滤保留可审查的索引策略
- **Then** 常用 `email`、`status` 或 `nickname` 查询 MUST 不依赖已删除的旧字段索引
