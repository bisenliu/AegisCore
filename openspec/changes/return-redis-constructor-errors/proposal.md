## Why

当前 Redis client 构造在 OpenTelemetry instrumentation 失败时直接 `panic`，会绕过 Fx constructor error、CLI 错误包装和统一退出策略。该问题还会让已创建的 Redis client 在错误路径上缺少可验证的关闭语义，并使测试难以注入 instrumentation failure。

## What Changes

- 将 `common/runtime/datastore` 的 Redis 普通 constructor 从 panic 语义调整为返回 `(*redis.Client, error)`。
- 将 Redis Fx provider 调整为传播 constructor error，并在错误中保留资源名称和原始 cause。
- 在 Redis tracing instrumentation 失败时关闭已创建 client，并通过错误链同时保留 instrumentation failure 与 close failure。
- 增加包内可注入 instrumentation seam，用于测试失败路径，不作为面向生产调用方的新公开能力。
- 增加覆盖 instrumentation 失败、client 关闭和 Fx constructor error 语义的测试。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: Redis datastore constructor 和 Fx provider 在 tracing instrumentation 失败时必须返回错误并关闭已创建 client，不得 panic。

## Impact

- 影响 `common/runtime/datastore/redis.go`、`common/runtime/datastore/redis_fx.go` 和相关测试。
- 影响 `user-service/internal/providers/redis.go` 及其调用签名，服务 Redis provider 需要传播 error。
- `OpenRedisClient` 的 Go API 返回值会新增 `error`，需要同步所有直接调用方。
- 不涉及 HTTP API、OpenAPI、数据库 schema、部署资产或外部依赖变更。
