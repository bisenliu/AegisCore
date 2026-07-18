## 1. Common Redis constructor

- [x] 1.1 更新 `common/runtime/datastore/redis.go`，让 `OpenRedisClient` 和 `openRedisClient` 返回 `(*redis.Client, error)`，并移除 instrumentation failure 的 panic。
- [x] 1.2 在 `openRedisClient` 中增加包内 instrumentation seam，默认调用 `redisotel.InstrumentTracing`，测试可临时替换并通过 cleanup 恢复。
- [x] 1.3 在 instrumentation 失败时关闭已创建 Redis client，并用错误链保留 `instrument redis tracing` cause 与 close failure。

## 2. Fx 与 user-service provider

- [x] 2.1 更新 `common/runtime/datastore/redis_fx.go`，让 `NewRedisClient` 返回 `(*redis.Client, error)`，constructor 失败时不注册 lifecycle hook 并返回包含资源名的错误。
- [x] 2.2 更新 `user-service/internal/providers/redis.go`，适配 `datastore.NewRedisClient` 新签名，并让 `cache_redis` provider error 保留资源名和原始 cause。
- [x] 2.3 搜索并更新 workspace 内所有 `OpenRedisClient` 和 `NewRedisClient` 调用点，确保签名变更无遗漏。

## 3. 测试

- [x] 3.1 增加或更新 `common/runtime/datastore` 测试，验证 instrumentation 失败返回 error 而非 panic。
- [x] 3.2 增加或更新 `common/runtime/datastore` 测试，验证 instrumentation 失败后已创建 Redis client 被关闭，并保留原始 cause。
- [x] 3.3 增加或更新 Fx provider 测试，验证 constructor error 包含 Redis 资源名并保留 instrumentation 原始 cause。
- [x] 3.4 更新 `user-service/internal/providers` 相关测试，验证 `NewCacheRedis` 对 `cache_redis` constructor error 的包装语义。

## 4. 验证

- [x] 4.1 运行 `go test ./runtime/datastore`，确认 common Redis datastore 测试通过。
- [x] 4.2 运行 `go test ./internal/providers`，确认 user-service provider 测试通过。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认 OpenSpec 和架构边界检查通过。
- [x] 4.4 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区。
- [x] 4.5 运行 `make lint`，确认 lint 通过。
- [x] 4.6 运行 `make verify`，确认完整验证通过且没有生成物 drift。
