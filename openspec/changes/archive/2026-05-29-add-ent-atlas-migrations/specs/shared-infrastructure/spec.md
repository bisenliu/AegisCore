## ADDED Requirements

### Requirement: Provide database connection inputs for migration tooling
共享基础设施配置必须能够为外部迁移工具提供目标 PostgreSQL 数据库连接信息。迁移工具可以使用部署环境提供的 `DATABASE_URL`，也可以从现有 `AEGISCORE_` 环境变量或 YAML PostgreSQL 命名实例配置组装等价连接 URL，但不得要求启动 Fx app、Redis client、HTTP server 或 Ent runtime client。

#### Scenario: Migration uses deployment database URL
- **WHEN** 部署环境提供 `DATABASE_URL`
- **THEN** Atlas 迁移执行必须能够使用该 URL 连接目标 PostgreSQL 数据库
- **THEN** 迁移执行不得依赖启动 Gin HTTP server、Redis client 或 Fx runtime graph

#### Scenario: Migration targets named user database
- **WHEN** 部署环境选择从项目配置组装数据库连接信息
- **THEN** 迁移执行必须使用用户服务拥有的 `postgre.user_db` 等价连接信息
- **THEN** 迁移执行不得因为配置中存在 `postgre.pay_db` 或 `postgre.common_db` 而迁移非目标数据库
