## ADDED Requirements

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
