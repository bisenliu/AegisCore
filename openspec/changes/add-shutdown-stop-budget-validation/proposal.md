  ✅dd d  

## Why

当前 `runtime.lifecycle.stop_timeout` 只校验不小于 HTTP 和 gRPC 的单个 shutdown timeout，但 Fx `OnStop` hooks 会串行、逆序执行并共享 `App.Stop(ctx)` 的总 deadline。HTTP/pprof drain、RBAC watcher、feature worker、auth purge pool、cache、Ent、Redis、PostgreSQL、tracing 和 logger 等前序 hook 可能耗尽共享 context，使后续清理无法获得新的完整预算，导致优雅关闭预算不可预测。

## What Changes

- 为 runtime lifecycle 增加组合停止预算校验，明确 `runtime.lifecycle.stop_timeout` 必须覆盖关键串行停止路径的最低预算。
- 将 auth purge worker 的停止等待上限纳入预算计算，避免隐藏的 30 秒 drain 与总 stop timeout 脱节。
- 为 user-service App lifecycle 增加 recorder 测试，验证关键关闭顺序和共享 deadline 语义。
- 明确 Fx `v1.24.0` 在某个 `OnStop` 返回 error 后仍继续执行其余 stop hooks，风险重点是共享 context 被前序 hook 耗尽，而不是普通 error 直接截断清理。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 增加共享 runtime 配置对 lifecycle 停止总预算的组合校验要求，并保持业务中立。
- `runtime-observability`: 增加 user-service 监听、worker、资源、tracing 和 logger 优雅关闭顺序与预算验证要求。

## Impact

- 影响 `common/runtime/config/defaults.go`、`common/runtime/config/validation.go` 的默认值和校验逻辑。
- 影响 `common/runtime/workerpool/pool.go` 或调用侧常量的 auth purge stop timeout 表达方式，但不得把 auth 业务语义放入 `common/runtime/workerpool`。
- 影响 `user-service/internal/bootstrap/server.go`、`user-service/cmd/serve.go` 的生命周期使用和测试覆盖。
- 影响 `user-service/internal/features/auth/infrastructure/redis/session_store.go` 的 purge pool 停止预算引用或测试约束。
- 不改变 HTTP API、OpenAPI、数据库 schema、migration、RBAC policy、部署清单或外部协议契约。
