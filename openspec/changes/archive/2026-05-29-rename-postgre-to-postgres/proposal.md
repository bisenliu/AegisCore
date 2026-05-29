## Why

当前运行时配置使用 `postgre` 作为 PostgreSQL 命名实例根节点，与通用产品和代码命名中的 `postgres` 不一致，容易在配置、环境变量和维护文档之间造成歧义。该变更将配置契约统一为 `postgres`，降低后续服务接入和运维配置成本。

## What Changes

- **BREAKING**: YAML 配置中的 PostgreSQL 命名实例根节点从 `postgre` 改为 `postgres`。
- **BREAKING**: `AEGISCORE_` 环境变量覆盖路径同步从 `AEGISCORE_POSTGRE_...` 改为 `AEGISCORE_POSTGRES_...`。
- 更新共享配置结构、加载测试、PostgreSQL provider 相关测试和用户服务配置样例，确保运行时读取 `postgres.<name>`。
- 更新用户服务运行时中声明 `user_db`、`common_db` PostgreSQL 连接池的配置来源描述。
- 更新迁移工具相关约定，使用 `postgres.user_db` 作为从项目配置组装目标数据库连接信息的命名路径。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: PostgreSQL 命名实例配置根节点从 `postgre` 更名为 `postgres`，并同步环境变量覆盖路径。
- `database-schema-migrations`: 从项目配置组装迁移目标数据库连接信息时，目标命名实例路径从 `postgre.user_db` 更名为 `postgres.user_db`。

## Impact

- 受影响代码：`common/config/`、`common/infrastructure/` 测试、`user-services/internal/bootstrap/` 测试、`user-services/configs/config.yaml`。
- 受影响配置：所有 YAML 中的 `postgre:` 根节点和 `.postgre_base` anchor 应改为 `postgres:` 与 `.postgres_base`。
- 受影响环境变量：PostgreSQL 配置覆盖前缀路径改为 `AEGISCORE_POSTGRES_<INSTANCE>_<FIELD>`。
- API 与数据模型无变化；HTTP 响应契约、Ent schema 和 SQL migration 文件不变。
