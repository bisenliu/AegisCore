# database-schema-migrations

## Purpose

数据库 schema 迁移能力为服务提供基于 Ent schema 和 Atlas SQL migration 的声明式迁移工作流，使数据库结构变更可以生成、审查、校验、打包并在部署前执行。

## Requirements

### Requirement: Maintain service-owned migration directories
系统必须将数据库迁移文件维护在拥有对应 Ent schema 的服务目录内。用户服务的迁移目录必须位于 `user-services/migrations/`，并必须包含生成的 `.sql` migration 文件和 Atlas 校验文件 `atlas.sum`。

#### Scenario: User service owns its migration files
- **WHEN** 用户服务的 Ent schema 发生数据库结构变更
- **THEN** 生成的 SQL migration 文件必须写入 `user-services/migrations/`
- **THEN** 对应的 `atlas.sum` 必须与 SQL migration 文件一起提交

#### Scenario: Service image contains only service migrations
- **WHEN** 构建用户服务 Docker 镜像
- **THEN** 镜像必须包含 `user-services/migrations/` 中的用户服务迁移文件
- **THEN** 镜像不得要求打包其他服务的迁移目录才能启动用户服务

### Requirement: Generate SQL migrations from Ent schema with Atlas
系统必须使用 Atlas 对比 Ent schema 与目标数据库状态生成 `.sql` migration 文件。迁移生成不得依赖服务运行时调用 `client.Schema.Create(ctx)`。

#### Scenario: Generate migration after Ent schema change
- **WHEN** 开发者修改 `user-services/ent/schema/` 下的 Ent schema 并运行迁移生成命令
- **THEN** Atlas 必须通过 Ent schema source 读取期望数据库结构
- **THEN** Atlas 必须在 `user-services/migrations/` 生成新的 `.sql` migration 文件
- **THEN** 生成流程必须更新或校验 `user-services/migrations/atlas.sum`

#### Scenario: Runtime schema create is not used for migrations
- **WHEN** 用户服务启动 HTTP runtime
- **THEN** 服务不得通过 `client.Schema.Create(ctx)` 自动创建或修改数据库 schema
- **THEN** 数据库 schema 变更必须通过已生成并部署的 SQL migration 文件完成

### Requirement: Allow reviewed manual SQL adjustments
系统必须允许开发者在提交前人工审查和调整 Atlas 生成的 SQL migration 文件。任何 SQL 文件内容变更后，系统必须要求重新计算 Atlas migration directory checksum。

#### Scenario: Adjust index creation for PostgreSQL
- **WHEN** 开发者将生成的普通索引 SQL 调整为 PostgreSQL 的 `CREATE INDEX CONCURRENTLY`
- **THEN** 调整后的 SQL 文件必须保留在 `user-services/migrations/` 中接受代码审查
- **THEN** 开发者必须重新生成 `atlas.sum`
- **THEN** CI 校验必须能发现 SQL 文件与 `atlas.sum` 不一致的情况

#### Scenario: Checksum mismatch blocks deployment
- **WHEN** `user-services/migrations/` 中的 SQL 文件被修改但 `atlas.sum` 未同步更新
- **THEN** Atlas migration directory 校验必须失败
- **THEN** CI/CD 或部署流程不得继续执行该迁移目录

### Requirement: Apply committed migrations before service startup
系统必须在用户服务 HTTP runtime 启动前执行已提交的 Atlas SQL migration，或由 CI/CD 在发布服务前完成迁移。迁移失败时服务启动流程必须停止。

#### Scenario: Apply migrations from deployment environment
- **WHEN** 部署流程提供目标 PostgreSQL 连接 URL 并执行迁移脚本
- **THEN** Atlas 必须从 `user-services/migrations/` 读取已提交 migration 文件
- **THEN** Atlas 必须将未应用的 migration 应用到目标数据库

#### Scenario: Migration failure prevents runtime startup
- **WHEN** Atlas 迁移执行失败
- **THEN** 容器 entrypoint 或 CI/CD release job 必须返回失败状态
- **THEN** 用户服务 HTTP runtime 不得在未完成必需迁移的情况下继续启动

### Requirement: Document repeatable service onboarding steps
系统必须为后续服务提供可复制的 Ent/Atlas 迁移接入步骤，包括目录结构、Atlas 配置、生成命令、人工修改规则和部署执行方式。

#### Scenario: New service adopts migration workflow
- **WHEN** 新服务引入自己的 Ent schema 和 PostgreSQL 数据库
- **THEN** 该服务必须在自身目录维护 Atlas 配置、schema loader 和 migrations 目录
- **THEN** 该服务必须复用相同的 SQL review、`atlas.sum` 校验和部署前 apply 规则
