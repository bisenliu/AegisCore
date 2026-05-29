## Context

当前 `common/infrastructure.NewRedisClient` 已支持按实例名从 `config.Config` 创建 Redis client，并注册启动 ping 与停止 close lifecycle。`common/infrastructure.Module` 不再固定提供 Redis client，因此具体服务必须像声明 PostgreSQL pools 一样，在自己的 Fx module 中声明需要的 Redis 实例。

`user-services/internal/bootstrap` 目前只通过 `NewPostgresPools` 声明 `user_db` 与 `common_db`，没有声明 `cache_redis`。这会导致用户服务运行时不连接 Redis，也无法让后续业务组件通过 Fx 注入具名 Redis client。

## Goals / Non-Goals

**Goals:**

- 在 `user-services` 模块中声明并提供具名 `cache_redis` Redis client。
- 复用 `common/infrastructure.NewRedisClient`，避免在用户服务中重复实现 Redis 连接、ping 或 close lifecycle。
- 保持 Redis 选择显式化：用户服务只连接声明的 `cache_redis`，不连接 `queue_redis`。
- 让启动失败语义保持一致：`cache_redis` 不可用时 Fx app 启动失败。

**Non-Goals:**

- 不新增缓存读写、队列、发布订阅或业务使用场景。
- 不修改现有 HTTP API、响应信封、错误码或路由。
- 不修改 Ent schema 或生成代码。

## Decisions

- 新增用户服务 Redis provider，例如 `NewRedisClients` 或 `NewRedisClient`，放在 `user-services/internal/bootstrap`。该 provider 使用 `fx.Out` 暴露 `*redis.Client`，注入名为 `cache_redis`。
- Redis 实例名使用常量 `cacheRedisName = "cache_redis"`，与配置文件 `redis.cache_redis` 和主规格中的命名实例保持一致。
- 不将 Redis client 注入 controller/service/repository，除非后续业务任务明确需要。当前变更只负责运行时依赖声明与可注入能力。
- 不连接 `queue_redis`。如果后续用户服务需要队列或发布订阅，应通过独立变更声明新的具名 Redis client。

## Risks / Trade-offs

- Redis 从“配置存在但不连接”变为“用户服务启动时连接 `cache_redis`” -> 测试和部署环境必须保证 `redis.cache_redis` 可用，否则启动会失败。
- 新增 Redis runtime dependency 但暂无业务消费者 -> 这是为用户服务显式使用 Redis 做准备，provider 测试会保证它可注入且 lifecycle 正确。
- `go-redis` 是 user-services 的间接依赖 -> 如果 user-services 代码直接引用 `*redis.Client` 类型，可能需要将依赖提升为直接 require，避免依赖声明不清晰。

## Migration Plan

1. 在 `user-services/internal/bootstrap` 新增 Redis provider，调用 `common/infrastructure.NewRedisClient` 创建 `cache_redis`。
2. 将 Redis provider 加入用户服务 Fx module 的 `fx.Provide`。
3. 更新 bootstrap 测试，验证 `cache_redis` 被提供、启动时 ping、停止时 close，且 `queue_redis` 不被提供或连接。
4. 如直接引用 `github.com/redis/go-redis/v9`，更新 `user-services/go.mod` 依赖状态。
5. 运行 `go test ./...` 于 `common/` 和 `user-services/`。

## Open Questions

- 无。
