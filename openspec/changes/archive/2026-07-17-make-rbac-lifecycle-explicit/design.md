## Context

`rbac-access-control` 当前已经定义了 policy reload、跨副本同步、user-role localcache 和 Fx 组合边界，但部分运行期资源仍在 permission infrastructure 内部通过 Fx lifecycle 或构造副作用启动。前置变更 `remove-fx-from-permission-adapters` 已完成后，permission infrastructure 应继续保留 Redis、Ent、Casbin 和 cache 的具体适配职责，但不再知道 Fx 的存在。

受影响路径集中在 `user-service/internal/features/permission/infrastructure` 及正式 permission/RBAC Fx module composition。变更不影响 `common`、HTTP API、OpenAPI 生成物、数据库 migration、部署清单或 metrics 标签；docs/openspec 只更新 `rbac-access-control` 规格的生命周期契约。

## Goals / Non-Goals

**Goals:**

- 将 Casbin initial load 改为显式 `Initialize` 或等价直接调用入口，由 composition 层按启动顺序调用。
- 将 RBAC watcher 改为构造与启动分离：`NewWatcher` 不启动 goroutine，`Start` 和 `Stop(context.Context)` 幂等并受 context deadline 约束。
- 将 user-role resolver/cache 改为启用和 disabled 模式都具备幂等 `Close` 契约，关闭只释放本地缓存资源，不关闭共享 Redis、Ent 或 PostgreSQL。
- 保持 initial load 失败后的 fail-closed、reload 状态、readiness 可观测性和既有同步语义。
- 移除 permission infrastructure 生产代码对 `go.uber.org/fx`、`go.uber.org/dig`、`fx.Lifecycle`、`fx.Hook`、`fx.In`、`fx.Out` 的依赖。

**Non-Goals:**

- 不实现顶层 Runtime、全服务 cleanup stack 或跨 feature 通用生命周期框架。
- 不改变 policy 派生规则、角色解析逻辑、Pub/Sub payload、Redis policy version 补偿、readiness 定义或 metrics 标签。
- 不改变 HTTP API、OpenAPI、数据库 schema、Atlas migration、部署资产或 `common` 共享契约。
- 不让 watcher 或 cache 负责关闭调用方注入的共享 Redis、Ent、PostgreSQL 资源。

## Decisions

- Decision: lifecycle 接口放在 permission infrastructure 的具体资源类型上，Fx hook 只留在正式 composition 层。
  Rationale: infrastructure 仍拥有 watcher/cache/engine 的资源细节，composition 只负责编排启动顺序和停止顺序，满足分层边界。
  Alternative: 保留 infrastructure 内部 `RegisterInitialLoad` 适配 Fx。拒绝原因是继续把 DI 框架依赖下沉到 adapter 边界。

- Decision: initial load 入口失败时返回错误但不把授权状态置为 allow。
  Rationale: 调用方可以观测启动错误并保持既有服务启动语义，同时 engine 的未加载或最近错误状态继续使授权 fail-closed。
  Alternative: initial load 失败直接 panic 或强制停止服务。拒绝原因是本次不改变既有启动语义和 readiness 定义。

- Decision: `NewWatcher` 只分配字段和校验依赖，`Start` 创建内部可取消 context 并启动长期循环，`Stop(ctx)` 取消并等待退出。
  Rationale: 构造副作用消失后，测试和 composition 都能证明启动顺序；`Stop(ctx)` 使用调用方 deadline 防止关闭无限阻塞。
  Alternative: 使用传入的 `OnStart` context 作为长期循环 context。拒绝原因是启动 context 生命周期短，不能表达服务运行期。

- Decision: user-role cache disabled 模式也提供 no-op `Close` 和 stats。
  Rationale: 调用方无需按配置分支处理关闭，禁用缓存仍保持直接回源和 fail-closed 授权语义。
  Alternative: disabled 模式返回 nil closer。拒绝原因是会把生命周期分支泄漏到 composition 层并增加遗漏关闭的风险。

## Risks / Trade-offs

- [Risk] 显式 lifecycle 调用顺序错误可能导致 watcher 在 initial load 前接收变更通知。→ Mitigation: Fx composition 按 initial load、watcher start、cache close/watch stop 的顺序登记，并用测试覆盖构造不启动和显式启动。
- [Risk] Stop 等待 goroutine 退出时可能因 Redis Pub/Sub 阻塞超过 deadline。→ Mitigation: `Stop(ctx)` 先取消内部 context，再在调用方 context 限制内等待；超时返回 context 错误且保持幂等可重试。
- [Risk] 移除 Fx adapter 后遗漏旧 import 或 lifecycle hook。→ Mitigation: 使用 `rg -n 'go\.uber\.org/(fx|dig)|fx\.(Lifecycle|Hook|In|Out)' user-service/internal/features/permission/infrastructure --glob '*.go' --glob '!**/*_test.go'` 验证无输出。
- [Risk] initial load 失败处理被误改为允许请求。→ Mitigation: 补充测试证明失败记录状态且后续授权 fail-closed。

## Migration Plan

1. 在 permission infrastructure 中新增或调整显式 lifecycle 方法，并移除 `RegisterInitialLoad(fx.Lifecycle, ...)` 及旧 Fx imports。
2. 更新正式 permission/RBAC Fx module composition，由组合层登记 initial load、watcher `Start`/`Stop`、cache `Close`。
3. 补充 initial load 失败、watcher 构造无 goroutine、重复 `Start`/`Stop`、`Stop` deadline、cache 重复 `Close` 和 disabled cache close 测试。
4. 运行 `cd user-service && go test ./internal/features/permission/... -count=1`、OpenSpec validate、架构 lint、lint 和 verify。

Rollback: 若实施后启动或关闭行为异常，可回退本 change 的代码与 specs，恢复旧 Fx lifecycle adapter；由于不涉及 schema、API 或部署资产，无需数据迁移回滚。

## Open Questions

无。
