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

### Requirement: Generate SQL migrations from Ent schema with Atlas
系统必须使用 Atlas 对比 Ent schema 与目标数据库状态生成 `.sql` migration 文件。迁移生成不得依赖服务运行时调用 `client.Schema.Create(ctx)`。

#### Scenario: Generate migration after Ent schema change
- **Given** 开发者修改 `user-services/ent/schema/` 下的 Ent schema
- **When** 开发者运行迁移生成命令
- **Then** Atlas 必须通过 Ent schema source 读取期望数据库结构
- **Then** Atlas 必须在 `user-services/migrations/` 生成新的 `.sql` migration 文件
- **Then** 生成流程必须更新或校验 `user-services/migrations/atlas.sum`

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

#### Scenario: Runtime schema create is not used for migrations
- **Given** 数据库 schema 变更已经通过 SQL migration 表达
- **When** 用户服务启动 HTTP runtime
- **Then** 服务不得通过 `client.Schema.Create(ctx)` 自动创建或修改数据库 schema
- **Then** 数据库 schema 变更必须通过已生成并部署的 SQL migration 文件完成

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
