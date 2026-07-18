## Context

本变更横跨 `common/runtime/observability/tracing` 和 user-service RBAC lifecycle。当前 tracing Fx provider 在 constructor 中以 `context.Background()` 创建 OTLP exporter，导致 exporter 阻塞时不受 `fx.StartTimeout` 控制，且 exporter 创建错误未保留 cause。RBAC 当前已经采用初始 policy load 失败后 fail-closed 的降级启动策略，但 `registerRBACLifecycle` 仍以 `if err := Initialize(...); err != nil` 表达，和真实语义不一致；同时 `OnStop` 在 watcher 与 user-role cache 同时关闭失败时只返回第一个错误。

## Goals / Non-Goals

**Goals:**

- 将 tracing exporter 初始化迁移到 Fx `OnStart(ctx)`，使启用 tracing 时的外部连接初始化受启动预算控制。
- 保留 OTLP exporter 初始化失败的底层错误 cause。
- 保持 tracing 禁用时不连接 exporter、非 nil provider 和 no-op/`NeverSample` 语义。
- 明确 RBAC 初始 policy load 的降级启动契约：启动成功、授权 fail-closed、readiness/startup 失败、后续 reload 成功后恢复。
- 在 RBAC lifecycle stop 中合并 watcher stop 与 user-role cache close 的全部错误。
- 用单元测试和 feature/module 级测试固定上述契约。

**Non-Goals:**

- 不改变 HTTP API、OpenAPI 注解或生成物。
- 不改变 Ent schema、Atlas migration 或持久化数据结构。
- 不改变 Redis policy version、Pub/Sub payload、metrics 名称、dashboard 或部署清单。
- 不新增兼容 constructor 路径来维持 tracing exporter 在 `fx.New` 阶段完成初始化的旧行为。

## Decisions

### tracing provider 采用 lifecycle 启动

启用 tracing 时，`NewFxProvider` 只构造 provider shell 并注册 lifecycle hook；OTLP exporter 和 SDK `TracerProvider` 在 `OnStart(ctx)` 中创建并写入 provider。`OnStop(ctx)` 调用现有 `Shutdown(ctx)`，使用 Fx 传入的停止 context。

备选方案是仅把 `context.Background()` 改为带固定 timeout 的 constructor context。该方案仍发生在 `fx.New` 阶段，不能被 `fx.StartTimeout` 统一治理，因此不采用。

### constructor 阶段不保留 ready 兼容契约

依赖 tracing 的构造器不得再依赖 `TracerProvider()` 在 `app.Start` 前已经连接真实 exporter。provider 的 `Tracer()`、`TracerProvider()` 和传播器访问需要在未启动状态下安全返回 no-op 或 nil-safe 结果；正式出站 Redis、Ent 和 HTTP middleware 在 `OnStart` 后使用真实 provider。

备选方案是在 constructor 创建 no-op provider，再在 `OnStart` 双写或兼容替换旧 ready 语义。该方案增加双状态兼容复杂度，不符合本次不保留兼容方案的要求，因此不采用。

### exporter 错误必须 wrapping

`newOTLPExporter` 对 `otlptrace.New` 的错误使用 `fmt.Errorf("create OTLP tracing exporter: %w", err)` 返回。这样启动日志和测试可保留底层 gRPC、TLS、endpoint 或 context 错误。

备选方案是仅改错误文本。该方案仍无法通过标准错误链定位 cause，因此不采用。

### RBAC 初始化显式命名为 fail-closed

Casbin engine 初始加载入口改为表达降级启动语义的 API，例如 `InitializeFailClosed(ctx)`，该方法不返回 reload 错误，而是把错误写入 `LastError`、reload metrics 和 fail-closed 状态。composition 层调用该方法后启动 watcher，不再保留不可达的 `Initialize` 错误分支。

备选方案是改为严格启动失败。当前主规格、健康检查和测试已围绕降级启动建立，严格启动会改变服务启动可用性策略，因此不采用。

### RBAC stop 使用 errors.Join

RBAC lifecycle `OnStop` 使用 `errors.Join(params.Watcher.Stop(ctx), params.Closer.Close())` 返回清理错误，使 Fx lifecycle 在汇总 hook errors 时能看到单个 hook 内的全部清理失败。

备选方案是记录 close error 但只返回 stop error。该方案仍丢失程序化错误链，不采用。

## Risks / Trade-offs

- tracing 依赖方若在 constructor 阶段直接解引用 SDK provider，可能暴露旧隐式 ready 契约。缓解方式：调整相关测试，确保 constructor 阶段只依赖 provider 对象和传播器安全性，真实 exporter 只在 `OnStart` 后可用。
- tracing exporter 初始化延迟到 `OnStart` 后，启动失败会发生在 `app.Start` 而非 `fx.New`。缓解方式：更新测试断言和启动错误排查文档，以 lifecycle 错误链为准。
- RBAC 降级启动会让 `app.Start` 成功但 readiness/startup 失败，运维需要以探针阻流而不是进程退出识别初始 policy 问题。缓解方式：新增集成测试覆盖 readiness 失败和后续恢复，保持健康检查 message 不泄露敏感信息。
- `errors.Join` 会改变 RBAC stop 返回错误文本形态。缓解方式：测试使用 `errors.Is` 检查各 cause，不依赖完整字符串。

## Migration Plan

1. 更新 tracing provider 状态管理、Fx provider lifecycle 和 exporter 错误 wrapping。
2. 更新 tracing 单元测试，删除 constructor 阶段 exporter ready 断言，新增 `OnStart(ctx)` 预算和 cause 保留测试。
3. 更新 RBAC Casbin 初始化入口命名或签名，调整 composition 调用并删除不可达 error branch。
4. 更新 RBAC stop hook 使用 `errors.Join` 并新增双错误测试。
5. 新增 RBAC 降级启动集成测试，覆盖初始 load 失败、watcher 运行、readiness/startup 失败、后续 reload 成功恢复、shutdown 清理。
6. 运行 `go test ./runtime/observability/tracing`、`go test ./internal/features/permission ./internal/features/permission/infrastructure/casbin ./internal/providers`，再运行 `make user-service-architecture-lint`。

回滚策略是整体回滚本 change 的代码和测试提交；本变更不涉及数据 migration、配置格式、部署清单或外部 API，因此无需数据回滚。

## Open Questions

无。
