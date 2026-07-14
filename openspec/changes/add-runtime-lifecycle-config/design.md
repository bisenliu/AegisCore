## Context

`aegiscore-user-services serve` 当前在 `user-service/cmd/serve.go` 中用 `fxAppStartTimeout = 15s` 和 `fxAppStopTimeout = 30s` 包裹 `app.Start` 与 `app.Stop`。这些值无法通过配置文件调整，导致不同运行环境无法按依赖初始化耗时、优雅关闭预算或后台 drain 需求设置合适的进程级生命周期预算。

现有共享配置由 `common/runtime/config` 定义默认值、加载和校验，user-service 通过 `user-service/internal/config` 适配服务私有配置。当前 change 只新增配置项并让 `serve` 使用配置值，不改变 HTTP server、scheduler、workerpool、数据处理、RBAC、认证或观测行为。

## Goals / Non-Goals

**Goals:**

- 新增 `runtime.lifecycle.start_timeout` 与 `runtime.lifecycle.stop_timeout` 配置字段。
- 保持缺省行为稳定，未配置时仍有明确启动和关闭超时。
- 移除 `user-service/cmd/serve.go` 中的 `fxAppStartTimeout` 和 `fxAppStopTimeout`，避免 CLI 层和配置层存在两套默认值来源。
- 让 `cmd/serve.go` 的 Fx app `Start` 和 `Stop` context timeout 使用配置值。
- 保持现有手动 `Start` / `Stop` 编排和 `context.WithoutCancel(upstreamCtx)` 的优雅关闭语义。
- 补充配置加载、校验、CLI 行为和文档测试覆盖。

**Non-Goals:**

- 不改用 `fx.App.Run()`。
- 不新增 CLI flag 覆盖 lifecycle timeout。
- 不新增 scheduler、workerpool、buffer flush 或任务级 shutdown 配置。
- 不改变 HTTP `server.http.shutdown_timeout` 或 gRPC `server.grpc.shutdown_timeout` 的含义。
- 不改变 HTTP API、OpenAPI、数据库 schema、部署资产、RBAC policy sync 或认证会话行为。

## Decisions

1. 将配置字段放在 `runtime.lifecycle`。

   `runtime.lifecycle` 表达进程级生命周期预算，区别于 `server.http.shutdown_timeout` 这类协议 server 级预算。备选方案是放在 `server` 下，但 Fx app 生命周期覆盖 provider、HTTP server、pprof、tracing、datastore、scheduler 和 workerpool 等多个组件，不应归属于单个协议 server。

2. 默认值只保留在配置层。

   `user-service/cmd/serve.go` 只负责消费已加载并校验过的 lifecycle timeout，不再定义 `fxAppStartTimeout` 或 `fxAppStopTimeout`。默认值统一由 `common/runtime/config` 的默认配置和 loader 维护。备选方案是在 `serve.go` 保留 fallback 常量，但这会形成双默认来源，增加 drift 风险。

3. 保留 `cmd/serve.go` 手动 `Start` / `Stop` 编排。

   当前 CLI 通过 Cobra `RunE` 返回启动或停止错误，并在 shutdown 时使用 `context.WithoutCancel(upstreamCtx)` 避免信号取消直接中断优雅关闭。备选方案是改用 `fx.Run()` 与 `fx.StartTimeout` / `fx.StopTimeout`，但这会改变 CLI 错误处理、测试替换方式和 shutdown context 语义，超出本次“只提取配置”的范围。

4. 在启动 app 前读取 lifecycle 配置。

   `app.Start` 的 timeout 需要在调用 `bootstrap.NewApp(configPath)` 后、`app.Start` 前可用。实现可复用现有配置 loader 读取完整配置或轻量读取 runtime lifecycle 字段，但不得引入与 Fx provider 内配置不一致的解析规则。备选方案是在 `bootstrap.NewApp` 内预先加载配置并传入 `fx.StartTimeout` / `fx.StopTimeout`，但当前配置是在 Fx provider 中加载，改动范围更大。

5. 配置值必须为正数。

   `0s` 不作为无限等待语义，避免生产发布、扩缩容或节点驱逐被无界 shutdown 阻塞。未配置字段通过默认值生效；显式配置非正数时应由配置校验拒绝。

6. 默认关闭超时必须不小于协议 server 关闭超时。

   `runtime.lifecycle.stop_timeout` 是 Fx app 总预算，必须覆盖 `server.http.shutdown_timeout` 和 `server.grpc.shutdown_timeout` 的默认值和显式配置，否则协议 server 的优雅关闭预算可能永远无法完整执行。

## Risks / Trade-offs

- 配置在 CLI 层和 Fx provider 内被读取两次 → 复用同一 loader 和默认值，避免解析规则分叉，并用测试覆盖默认值和显式配置。
- 默认值迁移后 `serve.go` 不再提供 fallback → 配置 loader 必须在返回配置前完成默认值填充和校验，测试必须覆盖缺省配置可启动路径。
- 用户误以为 `runtime.lifecycle.stop_timeout` 能保证内存数据一定落库 → 文档和规格明确该字段只提供 Fx app 总等待预算，不提供数据可靠性保证。
- stop timeout 设置过长导致发布变慢 → 保持正数校验但不强制业务默认过长，部署侧应与平台 termination grace period 对齐。
- stop timeout 小于 HTTP/gRPC shutdown timeout → 配置校验拒绝，避免总预算小于组件预算。

## Migration Plan

1. 在 `common/runtime/config` 增加 runtime lifecycle 配置结构、默认值、加载和校验。
2. 在 user-service 配置示例或开发文档中补充 `runtime.lifecycle.start_timeout` 与 `runtime.lifecycle.stop_timeout`。
3. 调整 `user-service/cmd/serve.go` 读取配置值并继续手动调用 `app.Start` / `app.Stop`，同时移除该文件中的 lifecycle timeout 默认常量。
4. 更新相关单元测试，覆盖默认值、显式配置、无效值和 serve stop context deadline。
5. 回滚时移除新增配置读取并恢复使用原常量；已有配置文件中的新增字段将不再被消费，但不影响数据库或外部 API。

## Open Questions

无。
