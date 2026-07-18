## Why

user-service 的 Fx composition root 当前没有启用 DI 初始化边界的 panic recovery，且部分 Fx 可达 constructor 将可预期的缺失依赖表达为 `panic`。这会让装配错误绕过标准错误返回路径，并掩盖 tracing provider 与 Redis client 等运行时资源在 constructor 阶段的初始化时序问题。

## What Changes

- **BREAKING**: 内部 application constructor 不再用 `panic` 表达可预期依赖缺失，改为返回明确 `error`，不保留旧 panic 语义。
- 修正 Fx tracing provider 的可用时序，使依赖 tracing 的 Redis、Gin、Ent 等 constructor 在 Fx graph 构造阶段获得非 nil provider。
- 修正 user-service cache Redis provider 对 tracing provider 的依赖检查和错误传播，Redis instrumentation 失败继续返回 error 而不是 panic。
- 在 user-service 顶层 `AppOptions` 启用 `fx.RecoverFromPanics()`，仅作为 Fx constructor、decorator 和 Invoke 的 DI 初始化边界保护。
- 增加针对 constructor error、Fx panic recovery 和 tracing/Redis 初始化路径的测试，明确该保护不替代 HTTP handler recovery、worker task recovery、后台 goroutine panic 策略或 lifecycle hook 运行期保护。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 调整 tracing provider、Redis instrumentation 和 Fx DI 初始化边界的稳定运行时行为。
- `shared-platform-primitives`: 调整共享 runtime tracing/datastore primitive 在 Fx adapter 下的 constructor 可用性和错误语义。
- `auth-session-management`: auth session lifecycle constructor 的缺失依赖由 panic 改为明确错误。
- `rbac-access-control`: permission/role command constructor 的缺失 policy change notifier 由 panic 改为明确错误。

## Impact

- 影响代码：`user-service/internal/bootstrap/app.go`、`common/runtime/observability/tracing/`、`common/runtime/datastore/`、`user-service/internal/providers/redis.go`、`user-service/internal/features/auth/application/sessions/`、`user-service/internal/features/permission/application/command/`、`user-service/internal/features/role/application/command/` 及对应测试。
- API 影响：无 HTTP API、OpenAPI 响应契约或路由变更。
- 数据库影响：无 Ent schema 或 Atlas migration 变更。
- 部署影响：启动期错误会以 Fx error 形式暴露，内部 constructor 缺失依赖不再产生未恢复 panic；顶层 Fx recover 只覆盖 DI 初始化边界。
- 观测影响：tracing provider 必须在 constructor 阶段提供稳定非 nil provider，并在 Fx lifecycle stop 时关闭；Redis command span 继续使用服务 tracing provider。
