## Why

当前 tracing exporter 在 Fx constructor 阶段使用 `context.Background()` 同步初始化，无法被 `fx.StartTimeout` 或启动 lifecycle context 约束；同时 OTLP exporter 构造失败时丢弃底层错误 cause，降低启动故障可诊断性。RBAC lifecycle 中 watcher 与用户角色缓存关闭错误未被完整合并，且初始 policy load 的 fail-closed 降级启动语义在装配层表达不够明确，容易让维护者误判启动失败契约。

## What Changes

- **BREAKING** tracing Fx provider 不再保证 `TracerProvider()` 在 `fx.New` constructor 阶段已经连接 exporter；启用 tracing 时 exporter 初始化迁移到 `OnStart(ctx)`，并受 Fx 启动预算控制。
- tracing OTLP exporter 构造失败 MUST 使用标准错误 wrapping 保留底层 cause，便于 `errors.Is`、`errors.As` 和日志排查。
- tracing `OnStop(ctx)` MUST 使用同一 lifecycle 停止 context 关闭 SDK provider，释放 exporter 资源。
- RBAC lifecycle `OnStop` MUST 合并 watcher stop 与 user-role cache close 的全部错误，不能因第一个错误丢弃后续清理错误。
- RBAC 初始 policy load 继续采用 fail-closed 降级启动：初始加载失败不阻断 `app.Start`，但 MUST 记录失败状态、拒绝授权、使 readiness/startup 失败，并允许后续 reload 成功后恢复。
- RBAC 初始化入口和装配层 MUST 通过命名、签名或结构化状态显式表达降级启动语义，不保留看似严格启动但不可达的兼容分支。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: tracing Fx provider 的 exporter 初始化、错误 cause 保留和 lifecycle 关闭契约发生变化。
- `rbac-access-control`: RBAC policy 初始化降级语义、readiness 恢复语义和 lifecycle stop 错误合并契约发生变化。

## Impact

- 影响 `common/runtime/observability/tracing/` 的 Fx provider、provider 启动状态管理、OTLP exporter 错误 wrapping 和相关单元测试。
- 影响 `user-service/internal/features/permission/fx.go`、`user-service/internal/features/permission/infrastructure/casbin/` 的初始化命名或签名、lifecycle stop 错误返回和相关测试。
- 影响 user-service 健康检查语义的测试覆盖：初始 policy load 失败时服务启动成功但 `/readyz`、`/startupz` 失败，后续 reload 成功后恢复。
- 不改变 HTTP API、OpenAPI 文档、数据库 schema、Atlas migration、部署资源、metrics 名称或 Redis policy version 协议。
