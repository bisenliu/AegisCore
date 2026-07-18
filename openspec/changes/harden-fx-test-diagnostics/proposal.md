## Why

当前部分 Fx 相关测试通过 `fx.NopLogger` 关闭 Fx event 日志，并且 RBAC watcher stop 等阻塞/关闭路径缺少测试级硬超时保护。该问题不会改变运行时功能，但会在依赖图构建、启动失败回滚、资源关闭阻塞等测试失败时降低可诊断性，并可能让单测卡到 `go test -timeout` 才失败。

## What Changes

- 移除 Fx 模块组合、生命周期顺序、启动失败回滚等测试中不必要的 `fx.NopLogger`，改用 `fxtest.New(t, ...)` 默认测试 logger 或 `fxtest.WithTestLogger(t)`。
- 保留负向构图测试的 `fx.New` 形态，但改用测试 logger，避免 `fxtest.New` 对预期 `app.Err()` 的测试提前 `FailNow`。
- 为 RBAC watcher stop、失败回滚和其它可阻塞关闭路径补充测试级硬超时，防止测试在 hook 或 Stop 实现不尊重 context 时无限等待。
- 对适合 `fxtest.NewLifecycle` 的生命周期 hook 单元测试启用 `fxtest.EnforceTimeout(true)`；对直接调用 `Stop(ctx)` 的测试使用 goroutine/select 保护。
- 不保留静默 Fx event 的兼容路径；只有明确断言错误且日志确实无价值的极少数负向测试可显式说明后继续静默。

## Capabilities

### New Capabilities

无。

### Modified Capabilities
- `delivery-operations`: 测试门禁要求 Fx 测试保留 event 诊断信息，并要求可阻塞关闭路径具备测试级硬超时保护。
- `runtime-observability`: 测试层面对 Fx lifecycle、pprof shutdown、tracing exporter shutdown 和资源关闭诊断行为提出更明确要求，不改变生产可观测性契约。
- `rbac-access-control`: 测试层面要求 RBAC watcher stop 和 policy sync 关闭回滚路径必须具备硬超时保护，不改变 RBAC 运行时业务行为。
- `auth-session-management`: 测试层面要求 auth session purge worker pool 关闭路径具备硬超时保护，不改变认证会话运行时业务行为。

## Impact

- 影响测试代码：`user-service/internal/features/role/fx_test.go`、`user-service/internal/features/permission/fx_test.go`、`user-service/internal/features/auth/fx_test.go`、`user-service/internal/bootstrap/validation_test.go`、`user-service/internal/bootstrap/http_test.go`、`user-service/internal/bootstrap/lifecycle_test.go`。
- 可能影响补充测试代码：`user-service/internal/features/permission/infrastructure/redis/watcher_test.go`、`user-service/internal/bootstrap/pprof_test.go`、`common/runtime/workerpool/pool_test.go`、`common/runtime/observability/tracing/provider_test.go`。
- 不影响 HTTP API、OpenAPI、数据库 schema、migration、部署资产或生产运行时配置。
- 验证重点为相关 Go package 测试和架构 lint；无需生成 OpenAPI 或 Atlas migration。
