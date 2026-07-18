## Context

当前 user-service 与 common 中存在多类 Fx 和关闭路径测试：模块依赖图构建、Fx app start/stop、启动失败回滚、HTTP/pprof drain、worker pool stop、tracing exporter shutdown、RBAC watcher stop。部分测试通过 `fx.NopLogger` 关闭 Fx event 输出，覆盖失败时只能看到最终错误，缺少 constructor、invoke、hook 和 rollback 事件线索。

RBAC watcher stop 等测试已经使用 context timeout 验证实现语义，但测试本身仍存在等待点：如果被测实现忽略 context 或 hook 永不返回，测试会阻塞到全局 `go test -timeout`。Fx v1.24.0 提供的 `fxtest.EnforceTimeout(true)` 只作用于 `fxtest.NewLifecycle`，不能直接保护 `fxtest.New` 创建的 App 或直接调用的 `Stop(ctx)`。

本 change 只调整测试诊断和测试级超时治理，不改变生产代码、HTTP API、数据库 schema、OpenAPI、部署资产或安全边界。

## Goals / Non-Goals

**Goals:**

- 让 Fx 组合测试默认输出测试 logger，失败时保留 Fx event 诊断信息。
- 让负向 Fx 构图测试继续断言 `app.Err()`，同时保留测试 logger。
- 为 RBAC watcher stop、pprof shutdown、worker pool stop、tracing exporter shutdown 等可阻塞测试增加测试级硬超时保护。
- 明确区分 `fxtest.EnforceTimeout(true)` 的适用范围和直接 `Stop(ctx)` 的保护方式。
- 保持测试改动最小化，不引入新的测试 helper 层级，除非重复模式明显影响可读性。

**Non-Goals:**

- 不修改生产运行时的 Fx lifecycle、RBAC watcher、worker pool、tracing provider、HTTP server 或 pprof server 行为。
- 不新增兼容开关或保留 `fx.NopLogger` 静默路径。
- 不改变 API、OpenAPI、Ent schema、Atlas migration、部署清单、Prometheus 指标或 Grafana dashboard。
- 不重构 feature 边界，不把测试 helper 移入 `common`、`internal/shared` 或 `internal/integration`。

## Decisions

- 使用 `fxtest.New(t, ...)` 默认测试 logger 作为正向 Fx app 测试的标准做法。备选方案是显式传入 `fxtest.WithTestLogger(t)`；默认方式更短，并且与 Fx 官方测试工具设计一致。
- 对需要检查 `app.Err()` 的负向测试继续使用 `fx.New`，并显式传入 `fxtest.WithTestLogger(t)`。备选方案是改用 `fxtest.New` 后捕获 `FailNow`，该方案会扭曲测试意图且不必要。
- 对直接测试 `fx.Lifecycle` hook 的场景使用 `fxtest.NewLifecycle(t, fxtest.EnforceTimeout(true))`。备选方案是手写 goroutine/select 包裹 lifecycle stop；Fx 官方选项更贴近被测对象且错误语义更清晰。
- 对直接调用 `watcher.Stop(ctx)`、`stopRBACLifecycle(...)`、`pool.Stop(ctx)` 或 pprof/http stop hook 的场景使用 goroutine/select 的测试级硬超时。备选方案是强行改造成 Fx lifecycle 测试，但会扩大改动范围并弱化直接单元测试的表达。
- 超时测试中的等待上限保持短而稳定：业务语义使用毫秒级 context timeout，测试硬超时使用约 1 秒级 guard，避免本地和 CI 偶发抖动。
- 代码归属保持在原测试文件内：RBAC watcher 测试留在 permission feature 或其 Redis infrastructure 测试；auth purge worker pool 测试留在 auth Redis infrastructure；runtime primitive 测试留在 common；bootstrap HTTP/pprof 测试留在 bootstrap。

## Risks / Trade-offs

- 风险：移除 `fx.NopLogger` 后失败测试输出会更详细，局部负向测试可能显得更吵。缓解：只在测试失败时输出更有价值；明确断言错误且日志无价值的场景可以保留局部静默说明，但本 change 不默认保留静默路径。
- 风险：测试级硬超时太短可能在慢 CI 上误报。缓解：实现语义 timeout 与测试 guard 分离，guard 使用足够宽松的秒级窗口。
- 风险：`fxtest.EnforceTimeout(true)` 被误用到 `fxtest.New` App 测试。缓解：tasks 中明确只用于 `fxtest.NewLifecycle`；App 测试通过 `Start(ctx)`/`Stop(ctx)` context 或 goroutine/select 保护。
- 风险：新增 helper 过度抽象降低测试可读性。缓解：优先就地使用小型 `done := make(chan error, 1)` 模式，只有多个文件重复明显时才抽取文件内 helper。

## Migration Plan

该 change 无生产部署迁移。实施顺序为先替换 Fx 测试 logger，再补充阻塞关闭路径硬超时，最后运行相关 package 测试和 `make user-service-architecture-lint`。

回滚方式为还原测试改动；由于不涉及生产代码、schema、API 或部署资产，回滚不需要数据迁移或发布顺序调整。

## Open Questions

无待决问题。当前上下文已确认需要直接落实不保留兼容方案。
