## Why

`user-services` 当前只显式声明 PostgreSQL 连接池，Redis 虽已作为共享基础设施能力存在，但用户服务运行时没有按命名实例选择并注入 Redis client。需要让用户服务明确使用 `redis.cache_redis`，使 Redis 依赖与 PostgreSQL 一样由服务模块声明，避免共享 module 隐式连接或业务代码无法消费 Redis。

## What Changes

- 在 `user-services/internal/bootstrap` 中声明用户服务需要的具名 Redis client，例如 `cache_redis`。
- 通过 `common/infrastructure.NewRedisClient` 按实例名创建 Redis client，并注册启动 ping 与停止 close lifecycle。
- 在用户服务 Fx module 中提供具名 `cache_redis`，供后续 controller/service/repository 或业务组件注入使用。
- 更新测试，验证用户服务会连接声明的 `cache_redis`，不会连接未声明的 `queue_redis`。
- 不新增 Redis 业务读写逻辑，不改变现有 HTTP API 响应。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 用户服务将声明并提供具名 Redis client，复用 common 中按实例名创建 Redis client 的基础能力。
- `http-service-runtime`: 用户服务启动时需要初始化声明的 Redis 依赖；Redis 不可用时启动失败，关闭时释放 Redis client。

## Impact

- 影响用户服务 Fx 装配：`user-services/internal/bootstrap/`。
- 影响共享基础设施使用路径：`common/infrastructure.NewRedisClient` 由用户服务模块调用。
- 影响运行时依赖：用户服务启动会连接 `redis.cache_redis`，但不连接 `redis.queue_redis`。
- 不新增 HTTP API、错误码、数据模型或 Ent schema；不修改 controller/service/repository 分层。
