## Why

`user-services/internal/bootstrap` 中部分 Fx provider 使用 `New*` 命名，但这些函数返回 `fx.Out` 形式的具名运行时依赖集合，职责更接近“向 Fx graph 提供服务侧依赖”而非普通对象构造。统一命名为 `ProvidePostgresPools`、`ProvideRedisClients` 和 `ProvideEntClients` 可以提升 bootstrap 装配代码的可读性和职责表达一致性。

## What Changes

- 将用户服务 bootstrap PostgreSQL pools provider 从 `NewPostgresPools` 重命名为 `ProvidePostgresPools`。
- 将用户服务 bootstrap Redis clients provider 从 `NewRedisClients` 重命名为 `ProvideRedisClients`。
- 将用户服务 bootstrap Ent clients provider 从 `NewNamedClients` 重命名为 `ProvideEntClients`。
- 更新 `user-services/internal/bootstrap/app.go` 的 Fx provider 列表和相关测试引用。
- 保持 `NewApp`、`NewJWTService`、`NewGinEngine`、`NewHTTPServer` 以及 controller/service/repository 构造函数命名不变。
- 不修改运行时配置、Fx name tags、数据库连接行为、Redis 连接行为、Ent schema、Atlas migration、HTTP API 或响应契约。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 标准化用户服务 bootstrap 中声明具名 Redis/PostgreSQL/Ent 运行时依赖的 Fx provider 命名；不改变运行时依赖契约或外部行为。

## Impact

- 影响代码：`user-services/internal/bootstrap/app.go`、`user-services/internal/bootstrap/postgres.go`、`user-services/internal/bootstrap/redis.go`、`user-services/internal/bootstrap/ent.go`、`user-services/internal/bootstrap/postgres_test.go` 及相关 bootstrap 测试引用。
- API 兼容性：不影响 HTTP API、错误码或响应格式。
- 配置兼容性：不影响 `postgres.user_db`、`postgres.common_db`、`redis.cache_redis` 或 `AEGISCORE_` 环境变量覆盖规则。
- 数据兼容性：不涉及 Ent schema、Atlas migration 或数据库数据变更。
- 依赖注入兼容性：保持 `user_db`、`common_db`、`cache_redis` 的 Fx named injection 标签和值不变。
