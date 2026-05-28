## Why

当前配置只支持单个 Redis 配置和共享 PostgreSQL 连接参数加多个数据库名，无法按服务清晰声明需要使用哪个 Redis 实例或 PostgreSQL 数据库实例。需要将 Redis 与 PostgreSQL 都调整为命名实例配置，让 `common` 保留基础 provider 能力，而具体服务在自己的模块中选择要连接的实例。

## What Changes

- 将 `user-services/configs/config.yaml` 中 Redis 配置改为带基础模板和命名实例的结构，例如 `redis.cache_redis`、`redis.queue_redis`。
- 将 PostgreSQL 配置从 `database.postgres` 调整为顶层 `postgre` 命名实例结构，例如 `postgre.user_db`、`postgre.pay_db`、`postgre.common_db`。
- 修改共享配置结构、校验和环境变量映射，使 Redis/PostgreSQL 命名实例都能从 YAML 与 `AEGISCORE_` 环境变量加载。
- 修改 `common/infrastructure`，保留 Redis 与 PostgreSQL 的基础连接创建代码，但不在共享 module 中固定连接所有实例。
- 修改 `user-services` 的 Fx 组装，由用户服务声明并选择需要的 Redis/PostgreSQL 命名实例。
- **BREAKING**：原有 `redis.*` 单实例字段和 `database.postgres.*` 配置路径将被新的命名实例配置路径替代。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 配置加载、Redis client provider、PostgreSQL pool provider 从固定单实例/共享数据库名模式调整为命名实例选择模式。

## Impact

- 影响配置文件：`user-services/configs/config.yaml`。
- 影响共享配置加载与校验：`common/config/`。
- 影响共享基础设施 provider：`common/infrastructure/`。
- 影响用户服务依赖声明与 Ent client 装配：`user-services/internal/bootstrap/`、`user-services/internal/entclient/` 及相关 Fx module。
- 不新增 HTTP API、错误码或数据模型；不引入支付业务运行时依赖。
