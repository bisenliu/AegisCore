## Context

`user-service/internal/features/permission/infrastructure/redis/watcher.go` 负责 RBAC policy sync 的 Redis Pub/Sub 监听和定时版本补偿。当前 `Watcher.Start(context.Context)` 接收 Fx `OnStart` context，但实际丢弃该 context，并创建内部 `context.WithCancel(context.Background())` 作为后台循环上下文；`Watcher.Stop(ctx)` 负责调用内部 cancel 并等待 goroutine 退出。

当前运行时关闭路径是正确的，但 `Start` 签名表达了错误预期：维护者可能以为启动 context 取消会停止 watcher。Fx `OnStart` context 只用于约束启动 hook 执行窗口，不应被误读为长期后台任务生命周期。

## Goals / Non-Goals

**Goals:**

- 将 `Watcher.Start` 改为无参数方法，明确启动动作不消费 Fx `OnStart` context。
- 保持后台循环由内部 cancel 控制、由 `Stop(ctx)` 关闭并等待 `done` 的既有运行时语义。
- 更新 Fx lifecycle hook 和 watcher 测试，使调用点与新签名一致。
- 通过聚焦测试证明 watcher 启动状态、正常停止和异常 channel 关闭记录错误的行为不变。

**Non-Goals:**

- 不改变 Redis policy version、Pub/Sub payload、周期性补偿检查或 Casbin reload 语义。
- 不调整 HTTP API、OpenAPI、数据库 schema、migration、部署清单或观测指标名称。
- 不引入 common/shared/integration 新抽象，不把 permission feature 内部 watcher 迁移到跨服务模块。
- 不保留 `Start(context.Context)` 兼容包装，也不新增旧签名适配层。

## Decisions

1. 删除 `Watcher.Start` 的 `context.Context` 入参。

   `Start` 仅负责幂等启动 watcher 后台循环，不需要也不消费外部 context。无参签名让调用方无法误传 Fx 启动 context，也让方法契约与实现一致。已排除保留旧签名并在注释中解释的做法，因为它仍会把无效参数暴露给维护者。

2. Fx `OnStart` hook 显式丢弃 context 后调用 `watcher.Start()`。

   Fx 仍会把启动 context 传入 hook，用于 hook deadline 和取消控制；本 change 只是不再把该 context 继续传递给 watcher 的长期 goroutine。`OnStop` 继续传递 Fx stop context 给 `watcher.Stop(ctx)`，用于限制等待后台循环退出的时间。

3. 后台循环上下文继续由 `context.WithCancel(context.Background())` 创建。

   watcher 的生命周期由 `Stop` 管理，符合当前运行时行为。`run(ctx, done)`、Redis subscribe、`Receive`、ticker 检查、payload 处理和 reload 调用继续使用内部 ctx，确保 `Stop` cancel 后能够统一收敛。

4. 测试只围绕新契约更新，不增加生产代码测试钩子。

   现有 watcher 测试已经覆盖启动/停止状态和异常 channel 关闭记录错误。本 change 更新调用签名，并补充或调整测试断言，证明取消 Fx 启动 context 不再是 watcher 契约的一部分，且 `Stop(ctx)` 仍能关闭后台循环。

## Risks / Trade-offs

- [Risk] 仓库内仍存在旧签名调用点导致编译失败 → 通过 `rg "Start(context.Context)|watcher.Start\\("` 检查调用范围，并运行目标包测试。
- [Risk] 修改签名被误认为改变运行时关闭行为 → 在 spec、design 和测试中明确 `Stop(ctx)` 仍是关闭入口，`Stop` context 仅限制等待退出时间。
- [Risk] OpenSpec delta 过度描述一次性重构 → 将要求限定在 RBAC policy watcher 生命周期契约，不扩大到 Redis policy sync 协议或 Casbin reload 行为。

## Migration Plan

本 change 是 user-service 内部 Go API 调整，不需要数据迁移或部署编排变更。实施时先更新 watcher 签名和 Fx hook，再更新测试调用点；若需要回滚，恢复本 change 对 `watcher.go` 和测试的修改即可。

## Open Questions

无。
