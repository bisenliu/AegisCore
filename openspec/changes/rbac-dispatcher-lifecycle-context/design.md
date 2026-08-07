## Context

RBAC outbox dispatcher 属于 permission feature 内的主动资源，负责在后台 claim PostgreSQL outbox event、发布 Redis 通知并 ack 或退避失败事件。当前 dispatcher 的启动接口是无参 `Start()`，实现会在 Start 路径内部使用 `context.Background()` 派生后台运行 context；permission Fx lifecycle hook 也只能调用无参 Start。

该模型让 dispatcher 后台循环、运行状态 metrics 与日志上下文脱离 Fx `OnStart(ctx)` 提供的生命周期上下文。Stop 侧已有 `Stop(ctx)` 用于等待后台循环退出，但该 ctx 的职责是停止等待期限，不应反向承担运行根上下文职责。本 change 只收紧 dispatcher 启动生命周期边界，不改变 outbox 投递、数据库事实、Redis 通知或 watcher/enforcer 上下文模型。

## Goals / Non-Goals

**Goals:**

- 将 `OutboxDispatcherRunner` 与 `Dispatcher.Start` 统一调整为 `Start(ctx context.Context) error`。
- `Dispatcher.Start(ctx)` 使用传入 ctx 派生后台运行 context，并保存 cancel 供 `Stop(ctx)` 触发退出。
- 后台轮询、运行状态 metrics 和结构化日志使用 dispatcher 生命周期 context 或其 logger-aware 派生 context。
- permission Fx lifecycle hook 在 `OnStart(ctx)` 中直接调用 `Runtime.Dispatcher.Start(ctx)`。
- 保持 start/stop 幂等：重复 `Start(ctx)` 不启动第二个 ticker，重复 `Stop(ctx)` 稳定返回。
- 用单元测试覆盖 dispatcher Start/Stop lifecycle 与 permission module lifecycle wiring。

**Non-Goals:**

- 不调整 outbox 数据库 schema、claim SQL、Ack/Fail 持久化语义或至少一次投递契约。
- 不改变轮询间隔、batch size、claim timeout、retry backoff 等配置语义。
- 不保留无参 `Start()` 兼容接口、adapter 或 fallback 分支。
- 不修改 Redis watcher、subscriber、Casbin enforcer 或 user-role cache 的上下文模型。
- 不改公开 HTTP API、OpenAPI、部署清单、数据库 migration 或观测资产定义。

## Decisions

1. 使用显式 `Start(ctx context.Context)` 替代无参 Start。

   理由：dispatcher 是 permission runtime 的主动资源，启动上下文应由 composition 层明确传入，避免 application 层实现自行选择 `context.Background()` 作为生命周期根。备选方案是保留 `Start()` 并在 Fx hook 内通过 adapter 捕获 ctx，但这会保留旧契约，且容易让后续调用方继续绕过 lifecycle context，因此不采用。

2. 在 `Dispatcher.Start(ctx)` 中派生内部可取消运行 context。

   理由：传入的 Fx `OnStart(ctx)` 可能带有启动期限，dispatcher 后台循环需要一个可由 `Stop(ctx)` 取消的内部 context，同时应继承启动 lifecycle context 中的 logger-aware 值。备选方案是直接保存传入 ctx 作为运行 ctx，但一旦 `OnStart(ctx)` 期限结束，后台循环可能被非预期取消；派生并保存 cancel 能保持停止职责清晰。

3. `Stop(ctx)` 继续只表达等待期限。

   理由：Stop 的 ctx 由 Fx 停止流程提供，用于限制等待 goroutine 退出的时间；运行根 context 应在 Start 时确定。备选方案是 Stop 时创建或替换运行 context，但这会混淆启动与停止语义，也无法让运行期间 metrics/logs 继承启动上下文。

4. metrics 与日志使用运行 context。

   理由：`DispatcherRunningObserved(true/false)` 和 loop 状态日志应与 dispatcher 生命周期绑定，便于统一 trace/logger-aware values。备选方案是继续在这些调用点使用 `context.Background()` 或 ad-hoc context，但会保留本 change 要消除的上下文断点。

5. 变更留在 permission feature 边界。

   理由：outbox dispatcher 是 user-service RBAC 同步的业务主动资源，不属于 `common` 的无业务语义 primitive，也不属于 `internal/shared` 或外部 integration。`common`、`deployments` 与 OpenAPI 生成物无需同步代码变更。

## Risks / Trade-offs

- [Risk] 如果直接使用带超时的 `OnStart(ctx)` 作为后台 loop ctx，启动完成后 loop 可能被取消。→ Mitigation：在 Start 中派生并保存内部 cancel，测试覆盖传入 ctx 与 Stop ctx 的职责分离。
- [Risk] 接口签名变更会导致测试 fake/mock 和 composition 编译失败。→ Mitigation：一次性更新所有 `OutboxDispatcherRunner` 实现、调用方和 fake/mock，不保留旧接口。
- [Risk] 重复 Start 时可能覆盖已有 cancel 或启动第二个 ticker。→ Mitigation：保留现有互斥与 running 判断语义，测试验证重复 Start 不创建第二个后台循环。
- [Risk] Stop 超时语义可能被误解为运行 context 替代物。→ Mitigation：在 spec 和测试中明确 Stop ctx 只限制等待退出，运行根 context 来自 Start(ctx)。

## Migration Plan

该 change 是内部 Go 接口与实现变更，不涉及数据库 migration、OpenAPI 生成、部署清单或线上数据迁移。实现时先更新 application 接口和 dispatcher，再更新 Fx lifecycle wiring、测试 fake/mock 与单元测试。

回滚方式是恢复本 change 前的代码版本；因为没有持久化格式和外部契约变化，回滚不需要数据迁移。但回滚会重新引入 dispatcher Start 路径使用 `context.Background()` 的旧行为。

验证方式：运行 `go test ./user-service/internal/features/permission/application ./user-service/internal/features/permission -run 'TestDispatcher|TestRegisterRBACLifecycle|TestPermissionModule'`。如实现过程中触及架构边界或文档规则，再运行 `make user-service-architecture-lint`。

## Open Questions

无。
