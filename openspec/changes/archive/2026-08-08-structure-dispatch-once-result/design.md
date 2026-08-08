## Context

RBAC policy outbox dispatcher 位于 `user-service/internal/features/permission/application`，负责 claim 已到期 event、发布 Redis policy revision 通知、ack 成功投递或记录失败退避。当前 `DispatchOnce(ctx)` 只返回 `error`，内部把 claim、逐条 dispatch、`Status(ctx)` backlog 刷新和最终错误聚合放在一个流程中。

本 change 只调整 permission application 内 dispatcher 的单次执行语义，不改变 outbox store 端口、持久化状态机、Redis payload、policy revision 协议、HTTP API、数据库 schema 或部署资产。

## Goals / Non-Goals

**Goals:**

- 为 `DispatchOnce` 提供结构化结果，使调用方和测试可区分 claim 失败、单条事件失败、ack/fail 失败、claim lost、backlog/status 刷新失败和 `ctx` canceled。
- 将 `DispatchOnce` 主流程拆成 claim batch、dispatch batch、refresh backlog/status、finalize result 四个私有步骤。
- 保持单条失败不阻塞同 batch 后续 claim 的可靠投递语义。
- 保持 `ctx` canceled 时不主动 Ack/Fail 当前未完成 claim，继续依赖 lease recovery。

**Non-Goals:**

- 不改变 `OutboxStore.Claim`、`Ack`、`Fail` 端口签名和持久化状态机含义。
- 不改变 outbox event payload、Redis policy refresh envelope 或 policy revision 发布协议。
- 不处理 dispatcher `Start(ctx)` lifecycle 上下文收敛问题。
- 不新增兼容旧 error-only 语义的 wrapper。

## Decisions

1. `DispatchOnce(ctx)` 返回 `DispatcherDispatchResult` 与 `error`。

   结构化结果记录 `Claimed`、`Delivered`、`Acknowledged`、`Retried`、`Failed`、`StatusRefreshed` 和 `Status`。返回 `error` 仍用于 `errors.Is`、日志和后台循环快速判断，但调用方以结果字段判断部分成功。备选方案是只定义结构化 error；该方案会把成功计数藏在错误类型里，成功场景也缺少明确结果。

2. 引入 `DispatcherDispatchError` 表达阶段化失败。

   错误包含 `Category`、`Stage`、`EventID`、`Revision` 和底层 `Cause`，并实现 `Unwrap()`。batch 内多个错误仍可 `errors.Join`，但每个错误有独立分类。备选方案是仅用字符串消息区分阶段；该方案难以稳定测试，也不利于后续调用方分支处理。

3. 单条事件 dispatch 返回小型私有结果。

   `dispatchClaim` 保持事件级处理职责，返回私有 `dispatcherClaimResult`，标记 delivered、acknowledged、retried、failed 和 error。batch 层负责累加计数和继续后续事件。这样不把 batch 统计泄漏到 publisher 或 store adapter。

4. backlog/status 刷新作为独立 finalize 前步骤。

   `refreshDispatchStatus` 调用现有 `Status(ctx)` 并只设置结果中的 `StatusRefreshed`、`Status` 和独立错误。它不会把已投递事件伪装成失败，也不会清除已有单条失败。成功清错仍由 finalize 判断：有 claim、无事件错误、无刷新错误时清除 dispatcher 最近错误。

## Risks / Trade-offs

- [Risk] `DispatchOnce` 签名变更会影响直接调用方和测试。→ Mitigation：当前调用方集中在 `run` 和单元测试，统一适配新返回值。
- [Risk] `errors.Join` 叠加结构化错误后测试断言可能过度依赖消息。→ Mitigation：测试优先断言结果字段、`errors.As` 和 `errors.Is`。
- [Risk] `Status(ctx)` 刷新失败会覆盖 `lastErrorCategory` 为 backlog。→ Mitigation：这是现有 `Status` 行为，结果中同时保留事件计数和独立 status refresh 错误，避免误读为事件全失败。
- [Risk] ctx cancel 中断 batch 后已成功事件和未完成 claim 混在同一次结果中。→ Mitigation：batch 层在每条 claim 前检查 `ctx.Err()`，返回已累计结果和 context error，不对未开始事件做 Ack/Fail。

## Migration Plan

这是 user-service 内部 application API 变更，无数据库、OpenAPI 或部署迁移。发布时随服务代码一起滚动即可。回滚方式是回退本 change 的代码和 OpenSpec change 文件。

验证方式：

- `openspec validate structure-dispatch-once-result --strict`
- `go test ./user-service/internal/features/permission/application -run 'TestDispatcherDispatchOnce|TestDispatcherRecords|TestDispatcherCancellation|TestDispatcherStatus'`
- `make user-service-architecture-lint`
