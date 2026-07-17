## MODIFIED Requirements

### Requirement: user-service 使用严格运行配置和部署契约

user-service 部署配置 MUST 使用当前严格配置契约：核心配置位于 `app/server/log/observability`，Redis 和 PostgreSQL 外部资源位于 `resources.redis` 和 `resources.postgres`，并且主 PostgreSQL 资源名 MUST 为 `primary_db`。

#### Scenario: 部署配置声明主 PostgreSQL 资源


- **WHEN** Compose、Kubernetes、Helm 或本地配置声明 user-service PostgreSQL 连接
- **THEN** 配置路径 MUST 使用 `resources.postgres.primary_db`
- **AND** 环境变量 MUST 使用 `AEGISCORE_RESOURCES_POSTGRES_PRIMARY_DB_*`
- **AND** 不得继续把 `resources.postgres.user_db` 描述为有效正向配置路径
