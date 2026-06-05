# database-schema-migrations

## Purpose

数据库 schema 迁移能力为服务提供基于 Ent schema 和 Atlas SQL migration 的声明式迁移工作流，使数据库结构变更可以生成、审查、校验、打包并在部署前执行。
## Requirements
### Requirement: Maintain service-owned migration directories
系统必须将数据库迁移文件维护在拥有对应 Ent schema 的服务目录内。用户服务的迁移目录必须位于 `user-services/migrations/`，并必须包含生成的 `.sql` migration 文件和 Atlas 校验文件 `atlas.sum`。

#### Scenario: User service owns its migration files
- **Given** 用户服务拥有 Ent schema 和服务内 migration 目录
- **When** 用户服务的 Ent schema 发生数据库结构变更
- **Then** 生成的 SQL migration 文件必须写入 `user-services/migrations/`
- **Then** 对应的 `atlas.sum` 必须与 SQL migration 文件一起提交

#### Scenario: Service image contains only service migrations
- **Given** 用户服务存在已提交的 SQL migration 文件
- **When** 构建用户服务 Docker 镜像
- **Then** 镜像必须包含 `user-services/migrations/` 中的用户服务迁移文件
- **Then** 镜像不得要求打包其他服务的迁移目录才能启动用户服务

#### Scenario: Name new migration files clearly
- **Given** 开发者为用户服务生成新的 SQL migration
- **When** 开发者为 migration 选择名称
- **Then** 名称必须描述 schema 变更语义，例如新增字段、索引或约束
- **Then** 系统不得要求重命名已经提交的 migration 文件或修改既有迁移历史

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

### Requirement: Allow reviewed manual SQL adjustments
系统必须允许开发者在提交前人工审查和调整 Atlas 生成的 SQL migration 文件。任何 SQL 文件内容变更后，系统必须要求重新计算 Atlas migration directory checksum。

#### Scenario: Adjust index creation for PostgreSQL
- **Given** Atlas 生成的 SQL migration 包含需要人工优化的索引语句
- **When** 开发者将生成的普通索引 SQL 调整为 PostgreSQL 的 `CREATE INDEX CONCURRENTLY`
- **Then** 调整后的 SQL 文件必须保留在 `user-services/migrations/` 中接受代码审查
- **Then** 开发者必须重新生成 `atlas.sum`
- **Then** CI 校验必须能发现 SQL 文件与 `atlas.sum` 不一致的情况

#### Scenario: Checksum mismatch blocks deployment
- **Given** `user-services/migrations/` 中存在已提交 SQL migration 文件和 `atlas.sum`
- **When** SQL 文件被修改但 `atlas.sum` 未同步更新
- **Then** Atlas migration directory 校验必须失败
- **Then** CI/CD 或部署流程不得继续执行该迁移目录

### Requirement: Apply committed migrations before service startup
系统必须在用户服务 HTTP runtime 启动前执行已提交的 Atlas SQL migration，或由 CI/CD 在发布服务前完成迁移。迁移失败时服务启动流程必须停止。

#### Scenario: Apply migrations from deployment environment
- **Given** 部署流程提供目标 PostgreSQL 连接 URL
- **When** 部署流程执行迁移脚本
- **Then** Atlas 必须从 `user-services/migrations/` 读取已提交 migration 文件
- **Then** Atlas 必须将未应用的 migration 应用到目标数据库

#### Scenario: Migration failure prevents runtime startup
- **Given** 容器 entrypoint 或 CI/CD release job 在服务启动前执行迁移
- **When** Atlas 迁移执行失败
- **Then** 容器 entrypoint 或 CI/CD release job 必须返回失败状态
- **Then** 用户服务 HTTP runtime 不得在未完成必需迁移的情况下继续启动

### Requirement: Derive migration targets from postgres named instances
当迁移工具从项目配置组装数据库连接信息时，系统必须使用 `postgres.<name>` 命名实例路径作为 PostgreSQL 配置来源。用户服务迁移目标必须解析为 `postgres.user_db`，不得继续依赖旧的 `postgre.user_db` 路径。

#### Scenario: User service migration derives target from postgres user database
- **Given** 用户服务配置包含 `postgres.user_db`、`postgres.common_db` 和 `postgres.pay_db`
- **When** 迁移执行脚本或辅助工具从项目配置组装目标数据库连接 URL
- **Then** 系统必须使用 `postgres.user_db` 作为用户服务 migration target
- **Then** 系统不得使用 `postgres.common_db`、`postgres.pay_db` 或旧的 `postgre.user_db` 作为用户服务 migration target

#### Scenario: Deployment database URL remains supported
- **Given** 部署环境提供 `DATABASE_URL`
- **When** Atlas 迁移执行脚本运行
- **Then** 系统必须允许迁移脚本直接使用 `DATABASE_URL`
- **Then** 系统不得要求启动 Fx app、Redis client、HTTP server 或 Ent runtime client

### Requirement: Document repeatable service onboarding steps
系统必须为后续服务提供可复制的 Ent/Atlas 迁移接入步骤，包括目录结构、Atlas 配置、生成命令、人工修改规则和部署执行方式。

#### Scenario: New service adopts migration workflow
- **Given** 新服务引入自己的 Ent schema 和 PostgreSQL 数据库
- **When** 新服务接入数据库 schema 迁移能力
- **Then** 该服务必须在自身目录维护 Atlas 配置、schema loader 和 migrations 目录
- **Then** 该服务必须复用相同的 SQL review、`atlas.sum` 校验和部署前 apply 规则

### Requirement: Migration naming guidance preserves history
数据库迁移相关命名标准化 SHALL 记录未来 migration 文件应使用清晰语义名称的约束，但不得重命名已存在 migration 文件、修改 `atlas.sum` 历史或改变数据库 schema。

#### Scenario: Existing migration filename is unclear
- **WHEN** 审查发现已存在 migration 文件名语义较泛
- **THEN** 实现 MUST 保留该文件名和迁移历史，并只在文档或规格中记录未来命名建议

#### Scenario: Migration capability is unaffected
- **WHEN** 命名标准化完成
- **THEN** Atlas migration 校验、生成脚本、apply 脚本和数据库结构 MUST 与修改前保持一致

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
系统 MUST 审查并更新用户表索引，使常用查询条件 `username`、`nickname`、`status` 和 `deleted_at IS NULL` 能与新的字段命名和软删除语义一致。`username` MUST 使用全表唯一约束，软删除后不得释放；`nickname` MUST 仅作为可重复展示名，不得建立唯一约束。

#### Scenario: Update indexes after field rename
- **Given** 用户表索引引用旧字段 `name` 或 `active`
- **When** migration 生成或人工审查 SQL
- **Then** 新索引 MUST 引用 `nickname` 或 `status`
- **Then** migration 完成后索引定义 MUST NOT 引用 `name` 或 `active`

#### Scenario: Preserve global username uniqueness with soft delete
- **Given** 用户表需要按用户名识别创建用户唯一账号名
- **When** migration 审查用户名唯一索引
- **Then** implementation MUST 使用全表 `UNIQUE(username)` 约束
- **Then** implementation MUST NOT 使用 `WHERE deleted_at IS NULL` 或等价条件的 partial unique index 释放已软删除用户名
- **Then** migration SQL 和实现说明 MUST 记录软删除后不释放 `username` 的全局唯一策略
- **Then** repository 的创建冲突处理 MUST 与全表唯一索引语义一致

#### Scenario: Prevent duplicate lowercase usernames before adding constraint
- **Given** 目标数据库中可能存在大小写不同但小写后相同的 `username`，或软删除记录与未删除记录使用相同 `username`
- **When** 开发者审查用户名全局唯一 migration
- **Then** migration review MUST 明确冲突检测或部署前数据清理策略
- **Then** migration MUST NOT 静默创建会导致唯一约束失败或账号归属不明确的数据状态

#### Scenario: Index active user lookup paths
- **Given** 查询、列表和登录默认只访问未软删除用户
- **When** migration 审查索引
- **Then** 系统 MUST 为 `deleted_at` 相关过滤保留可审查的索引策略
- **Then** 常用 `username`、`status` 或 `nickname` 查询 MUST 不依赖已删除的旧字段索引

### Requirement: Organize Ent schema files by domain without changing database schema
系统 SHALL 将用户服务 Ent schema 源文件实际组织为可扩展的领域分类。当前 `User` schema MUST 纳入用户领域分类，并且根 `schema` package MUST 继续作为 Ent codegen 和 Atlas schema source 的稳定入口。该分类变更 MUST NOT 改变 `User` 字段、索引、字段注释、表结构或 migration 历史。

#### Scenario: User schema is moved into domain classification
- **Given** 当前用户服务仅包含 `User` Ent schema
- **When** 本次结构重构完成
- **Then** `User` schema MUST 位于用户领域分类路径中
- **Then** 根 `user-services/ent/schema` package MUST 继续向 Ent codegen 暴露 `User` schema
- **Then** `User` 字段、索引和注释语义 MUST 与分类前保持一致
- **Then** 本次变更 MUST NOT 因目录规划生成新的数据库 migration

#### Scenario: Ent generation reads categorized schema
- **Given** `User` schema 已移动到领域分类路径
- **When** 开发者在 `user-services` 模块运行 `go generate ./ent`
- **Then** Ent codegen MUST 成功读取根 `schema` package 暴露的 `User` schema
- **Then** 生成的 `user-services/ent/` 代码 MUST 与 `User` 字段和索引语义保持一致
- **Then** 实现 MUST NOT 手写 `user-services/ent/` 下的生成代码表达 schema 分类变更

#### Scenario: Atlas source remains stable after schema categorization
- **Given** Ent schema 已按领域分类且根 `schema` package 保持聚合入口
- **When** Atlas 使用 `ent://ent/schema` 或等价配置读取用户服务 schema source
- **Then** Atlas MUST 能读取完整 `User` schema
- **Then** 未发生字段或索引语义变更时 MUST NOT 生成新的数据库结构 migration

### Requirement: Avoid Ent schema subpackage ambiguity with generated packages

系统 SHALL 在按领域拆分用户服务 Ent schema source 时，为 `user-services/ent/schema/` 下的 schema 子包使用能明确表达 schema source 语义的包名。schema 子包名称 MUST 避免与 Ent codegen 生成的查询包名称混淆；根 `user-services/ent/schema` package MUST 继续作为 Ent codegen 和 Atlas schema source 的稳定入口。该命名优化 MUST NOT 改变 `User` 字段、索引、字段注释、表结构或 migration 历史。

#### Scenario: User schema subpackage uses explicit schema name
- **WHEN** 用户服务将 `User` schema 的字段和索引定义保留在根 `schema` package 之外的子包中
- **THEN** 子包名称 MUST 明确表达 schema source 语义，例如 `userschema`
- **THEN** 子包名称 MUST NOT 与 Ent 生成查询包 `github.com/aegiscore/user-services/ent/user` 同名为 `user`

#### Scenario: Ent generation remains stable after schema subpackage rename
- **WHEN** 开发者在 `user-services` 模块运行 `go generate ./ent`
- **THEN** Ent codegen MUST 继续通过根 `user-services/ent/schema` package 读取完整 `User` schema
- **THEN** 实现 MUST NOT 手写 `user-services/ent/` 下的生成代码表达该命名优化

#### Scenario: Schema subpackage rename does not create migration
- **WHEN** 本次命名优化仅移动或重命名 schema source 子包且未改变字段、索引、字段注释、默认值或约束
- **THEN** `user-services/migrations/` MUST NOT 因本次变更新增 SQL migration
- **THEN** 既有 `atlas.sum` MUST NOT 因无数据库语义变化而修改

### Requirement: Preserve schema semantics when decoupling default value sources

系统 SHALL 在仅调整 Ent schema 默认值来源的重构中保持数据库 schema 语义不变。若字段类型、默认值数字、注释、索引和约束未变化，系统 MUST NOT 生成新的 Atlas SQL migration 或修改既有 migration 历史。

#### Scenario: User status default source changes without migration
- **Given** `User` Ent schema 的 `status` 字段默认值来源从业务 domain 常量改为 schema 本地持久化契约值
- **When** 开发者审查本次 schema source 变更
- **Then** `status` 字段的数据库默认值 MUST 继续为 `100`
- **Then** 用户表字段类型、字段注释、索引和约束 MUST 保持不变
- **Then** `user-services/migrations/` 中 MUST NOT 因本次默认值来源重构新增 SQL migration
- **Then** 既有 `atlas.sum` MUST NOT 因无数据库语义变化而修改

#### Scenario: Ent generation remains the only generated-code path
- **Given** Ent schema 默认值来源已与业务 domain 解耦
- **When** 开发者需要刷新 Ent 生成代码
- **Then** 开发者 MUST 在 `user-services` 模块运行 `go generate ./ent`
- **Then** 实现 MUST NOT 手写 `user-services/ent/` 下的生成代码
- **Then** 生成结果 MUST 与用户表 `status` 字段的既有数据库默认值语义保持一致
