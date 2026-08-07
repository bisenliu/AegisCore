## Context

permission Casbin Engine 负责维护本实例内存 policy projection，并通过 revision-aware reload 将本地 enforcer 推进到数据库权威 revision。当前并发 reload 通过 shared flight 合并实际加载工作，多个 `ReloadToRevision(ctx)` 或 `RefreshToRevision(ctx)` 调用方可以等待同一个 flight，且单个 waiter context 取消不应取消其他 waiter 仍需要的共享 reload。

本变更关注 shared flight 的 root context 归属。flight 不能继续依赖 `context.Background()` 这类进程级不可取消 root；它必须从 Casbin Engine 的 lifecycle root 派生，使 Fx/RBAC runtime stop、启动回滚和服务 shutdown 能取消正在阻塞的 policy loader。该变更仅属于 user-service permission infrastructure 和 RBAC lifecycle 装配，不进入 `common`、`internal/shared` 或 `internal/integration`。

## Goals / Non-Goals

**Goals:**

- 让 Casbin Engine 在启动边界显式建立 lifecycle root context，并让 shared reload flight 从该 root 派生。
- 保留 waiter context 与 shared flight context 的职责边界：waiter context 只控制当前调用是否继续等待，不能取消共享 reload 工作。
- 让 engine lifecycle root 取消时，正在阻塞的 shared reload loader 能收到取消信号并进入 fail-closed 状态。
- 保持 `InitializeFailClosed(ctx)` 使用 target revision 0，且初始化失败不阻断 `app.Start`、后续授权继续 fail-closed。
- 更新测试覆盖并发 coalescing、单 waiter 取消、全部 waiter 取消、唯一 waiter 取消后新 flight、lifecycle root 取消和初始化 ctx 语义。

**Non-Goals:**

- 不把任意单个 `ReloadToRevision(ctx)` 或 `RefreshToRevision(ctx)` 调用方 context 作为 shared flight root。
- 不改变 policy loader 查询语义、Casbin model、permission rule 映射或 user-role resolver cache 策略。
- 不改变 Enforce fail-closed 行为、HTTP API、OpenAPI、数据库 schema、Redis protocol、outbox event 契约或部署清单。
- 不在 `common`、`internal/shared` 或 `internal/integration` 新增 RBAC 生命周期抽象。
- 不保留旧 `context.Background()` flight 兼容路径或 fallback 分支。

## Decisions

- Engine 拥有 lifecycle root context：在 `Engine.Start(ctx)` 中保存 `context.WithCancel(ctx)` 派生的 root，在 `Engine.Stop(ctx)` 中取消 root 并标记 lifecycle stopped。理由是 engine 是 reload flight 的资源所有者，lifecycle root 应由资源 owner 管理。备选方案是在 provider 中直接把 root context 注入 `ReloadToRevision`，但这会把 shared flight root 与单次调用等待语义混在一起。
- `startFlightLocked` 只从 engine lifecycle root 派生 shared flight context：新的 flight 使用 `context.WithCancel(e.lifecycleCtx)`，不得使用 `context.Background()`。理由是服务 shutdown 和启动回滚必须能取消 loader，同时保留 flight 内部 cancel 用于 flight 完成或所有 waiter 取消后的清理。备选方案是每个 waiter 加入时把自己的 context 合并进 flight，但任一 waiter 取消会影响其他 waiter，违背现有语义。
- waiter context 只影响当前等待：当某个 waiter context 取消时，该调用返回取消错误并从 flight waiter 计数中移除；只要仍有其他 waiter，shared flight 继续运行。理由是并发 reload 的实际工作服务于全局 projection，不属于单个调用方。备选方案是只要有 waiter 取消就取消 shared flight，但会让短超时调用破坏其他调用与副本收敛。
- 全部 waiter 取消时可以取消当前 shared flight：当 waiter 计数降为 0 时取消 flight，记录失败并保持 fail-closed。理由是此时没有调用方继续等待结果，继续阻塞 loader 缺乏请求侧约束；后续新 waiter 应启动 fresh flight。备选方案是即使全部 waiter 取消也继续运行到完成，但会增加无用工作和 shutdown 排查噪音。
- RBAC Fx lifecycle 先启动 engine root，再执行 fail-closed 初始化，再启动 watcher 和 dispatcher；停止时先停止 dispatcher，再取消 runtime root 并停止 watcher/engine。理由是 watcher/dispatcher 依赖 engine root 已存在，停止时应先阻止新后台投递，再取消根 context 使主动资源尽快退出。备选方案是 watcher 启动后再初始化，但会让 watcher 与初始化 reload 竞争，增加启动状态不确定性。

## Risks / Trade-offs

- [Risk] lifecycle root 被过早取消会使正在进行的 reload 失败并保持 fail-closed → Mitigation：仅由 RBAC lifecycle Stop 或启动回滚取消 root，并通过测试覆盖 root 取消传播与 status 错误。
- [Risk] `Start`/`Stop` 幂等边界处理不当可能导致停止后还能启动 flight → Mitigation：engine 在 reload 前检查 lifecycle started，Stop 标记 stopped，并测试未启动或已停止状态下 reload 的失败语义。
- [Risk] 全部 waiter 取消后记录 `context.Canceled` 可能使 readiness 暂时 fail-closed → Mitigation：这是安全预期；后续显式 reload、watcher 或周期补偿成功后清除错误并恢复 readiness。
- [Risk] 修改 lifecycle wiring 可能影响 Fx 启停顺序 → Mitigation：保留现有 RBAC runtime 聚合对象和 hook 顺序，只在 engine root 控制面内完成变更，并用 permission lifecycle 单元测试验证 start/stop/rollback 顺序。

## Migration Plan

该变更不需要数据库 migration、OpenAPI 生成、部署清单或观测资产迁移。发布时随 user-service 二进制滚动部署即可；新副本启动后使用新的 engine lifecycle root 控制 reload flight。

回滚策略是回滚 user-service 代码版本。由于没有 schema、API 或协议变化，回滚不需要数据修复。回滚后旧版本可能重新使用不可由 engine lifecycle root 取消的 shared reload root，这是可观测性和 shutdown 行为退化，不影响持久数据兼容性。

验证方式包括运行 `go test ./user-service/internal/features/permission/infrastructure/casbin -run 'TestEngineWaiterCancellation|TestEngineNewWaiter|TestEngineCoalesces|TestEngineFaultInjection|TestEngineInitialize'`，以及覆盖 permission lifecycle wiring 的相关测试。合并前按仓库规则运行相关包测试，必要时运行 `make user-service-architecture-lint` 和 `make verify`。

## Open Questions

无。
