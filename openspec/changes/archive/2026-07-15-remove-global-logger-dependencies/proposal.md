## Why

当前 `common/runtime/logger` 的构造函数会无条件覆盖进程级默认 logger，使单独构造 logger 的测试、CLI 或多个 Fx App 在同一进程中并行装配时可能互相污染日志实例。user-service 的 user、auth、role、permission application 与部分关键 infrastructure 仍在正式主路径依赖可变进程全局默认 logger，导致依赖边界不可追踪，也削弱并行测试隔离性。

本变更通过移除 logger 构造副作用并显式注入业务日志依赖，使正式 App 的日志实例归属可从构造函数或 request context 追踪，同时保留 common 层必要的无注入兜底 API。

## What Changes

- 修改 `common/runtime/logger.New`、`NewWithConfig` 和 `NewLogger` 的稳定语义：只构造并返回 `*zap.Logger`，不再隐式调用 `SetDefault` 或覆盖进程级默认 logger。
- 保留正式 logger 的既有 Sync 关闭责任，由当前 lifecycle owner 在 App Stop 阶段继续同步，不新增默认 logger 的 Fx lifecycle owner、互斥锁协议或安装/恢复机制。
- 调整 user-service 的 user、auth、role、permission application 和关键 infrastructure：日志依赖通过 constructor 参数或明确 request context logger 路径传递，正式业务主路径不再从 package-level 默认 logger 获取可变依赖。
- 保留 `common/runtime/logger` 中确有必要的无注入兜底 API，用于共享 helper 或非正式主路径 fallback；正式 user-service App 不安装、不恢复也不持有默认 logger。
- 增加架构检查或静态约束，防止 feature application 重新以 package-level 默认 logger 作为正式主路径依赖。
- 更新日志依赖约束相关规格，覆盖 `user-identity-management`、`auth-session-management`、`rbac-access-control`、`shared-platform-primitives` 和 `runtime-observability`。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 约束 `common/runtime/logger` 构造函数不得隐式覆盖进程级默认 logger，并保留 common 层受限兜底能力。
- `runtime-observability`: 约束正式 App 的 logger 生命周期、Sync 责任、context logger 传播和全局默认 logger 使用边界。
- `user-identity-management`: 约束 user feature application 和关键 infrastructure 的日志依赖必须显式注入或来自 request context。
- `auth-session-management`: 约束 auth feature application、session/token/credential 相关关键 infrastructure 的日志依赖必须显式注入或来自 request context。
- `rbac-access-control`: 约束 role 与 permission feature application、policy sync、Casbin/Redis/PostgreSQL 关键 infrastructure 的日志依赖必须显式注入或来自 request context。

## Impact

- Affected code: `common/runtime/logger/`，`common/http/...` 中使用 logger context 的共享 HTTP helper，`user-service/internal/providers/`，以及 `user-service/internal/features/user|auth|role|permission/` 的 application、infrastructure、composition provider 和测试 fixture。
- API impact: Go constructor 语义变化属于内部共享 Go API 行为变更；不改变 HTTP API、响应 envelope、request ID、trace ID、OpenAPI 文档或外部传播契约。
- Data impact: 不修改 Ent schema、Atlas migration、PostgreSQL/Redis 数据结构或 RBAC policy 数据契约。
- Observability impact: 日志字段名、日志级别、logger name、`component`、`request_id`、`trace_id`、`span_id` 和敏感信息过滤契约保持不变；变化点是日志实例来源从可变全局默认转为显式依赖。
- Testing and tooling impact: 需要更新单元测试 fixture、并行 App 构造测试和架构 lint；验证包含 `cd common && go test ./runtime/logger ./http/... -count=1`、四个 feature application/infrastructure 测试、`make user-service-architecture-lint`、`openspec validate remove-global-logger-dependencies`、暂存预期变更后的 `make lint` 与 `make verify`。
