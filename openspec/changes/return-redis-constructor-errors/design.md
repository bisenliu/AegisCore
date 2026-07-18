## Context

`common/runtime/datastore` 当前用 `OpenRedisClient` 和 `NewRedisClient` 构造单个 Redis client，并在 constructor 中调用 `redisotel.InstrumentTracing`。当 instrumentation 返回错误时，当前实现直接 `panic`，导致 `fx.New` 可能绕过 constructor error 返回路径，CLI 无法统一追加错误上下文，也无法验证已创建 client 是否关闭。

本次变更横跨 `common` Redis datastore primitive 与 `user-service` Redis provider 组装，但不改变业务 API、数据库结构、OpenAPI、部署清单或观测资产契约。

## Goals / Non-Goals

**Goals:**

- Redis 普通 constructor 和 Fx provider 在 instrumentation 失败时返回 error，不再 panic。
- instrumentation 失败时关闭已创建 client，并在错误链中保留 instrumentation failure 和 close failure。
- user-service `cache_redis` provider 继续只选择服务私有资源名，并传播包含 `cache_redis` 的构造错误。
- 增加可测试的包内 seam，覆盖 instrumentation failure、client close 和 Fx constructor error。

**Non-Goals:**

- 不新增 Redis 业务 key、缓存策略、健康检查、metrics 或 tracing span 语义。
- 不引入 `fx.RecoverFromPanics()` 作为主要修复手段。
- 不修改 HTTP API、OpenAPI、Ent schema、Atlas migration、Kubernetes/Compose/Helm 部署资产。
- 不为测试暴露新的全局公开 API、`NewXForTest` 或服务级 mock 仓库。

## Decisions

- Redis constructor 改为显式返回 error。
  备选方案是保留 panic 并依赖 `fx.RecoverFromPanics()`。该方案只能把 panic 作为兜底恢复，不能表达普通 Go 调用的错误语义，也不保证错误路径关闭 client，因此不采用。

- `openRedisClient` 在创建 client 后执行 instrumentation，失败时返回 `errors.Join(fmt.Errorf("instrument redis tracing: %w", err), client.Close())`。
  备选方案是只返回 instrumentation error。该方案会遗漏 close failure，不利于诊断资源释放问题，因此不采用。

- `NewRedisClient` 返回 `(*redis.Client, error)`，只有 constructor 成功后才注册 lifecycle hook。
  备选方案是在 hook 中延迟执行 instrumentation。该方案会把 constructor 失败推迟到 `App.Start`，并改变当前 Redis client 构造时机，因此不采用。

- user-service `NewCacheRedis` 捕获 `datastore.NewRedisClient` error，并包装资源名 `cache_redis` 后返回。
  备选方案是完全依赖 common provider 内部资源名上下文。由于服务 provider 是具名资源选择边界，错误中显式保留服务资源名更利于 CLI 和 Fx graph 诊断。

- instrumentation seam 留在 `common/runtime/datastore` 包内，作为非导出的函数变量或小型非导出函数参数注入点。
  备选方案是新增公开 option 暴露 instrumentation function。该方案会把测试便利扩张为生产 API，不符合共享 primitive 边界，因此不采用。

## Risks / Trade-offs

- `OpenRedisClient` 签名变更会影响直接调用方 -> 同步更新 `common` 测试和所有 workspace 调用点，并运行相关包测试。
- `NewRedisClient` 签名变更会影响 Fx provider 调用方 -> 更新 `user-service/internal/providers/redis.go` 包装逻辑，并验证 Fx provider 测试。
- `errors.Join` 可能改变错误字符串顺序或包含多 cause -> 测试使用 `errors.Is`、`require.ErrorContains` 和稳定资源名断言，避免依赖完整错误文本。
- 包内 seam 若设计不当可能引入并发污染 -> 测试通过 `t.Cleanup` 恢复默认 instrumentation function，相关测试不并行执行。

## Migration Plan

- 先更新 `common/runtime/datastore` Redis constructor 签名和错误处理。
- 再更新 `user-service` Redis provider 对新签名的调用和错误包装。
- 同步更新或新增 `common/runtime/datastore` 与 `user-service/internal/providers` 测试。
- 运行相关 Go 测试；本变更不需要 migration、OpenAPI 生成或部署资产生成。

回滚时恢复 Redis constructor 签名与调用点即可；由于不涉及持久化数据、外部 API 或部署资源，回滚不需要数据迁移。

## Open Questions

- 无。
