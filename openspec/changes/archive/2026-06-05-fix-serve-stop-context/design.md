## Context

`user-services/cmd/main.go` 的 `runServe` 接收 Cobra 传入的上游 `context.Context`，随后通过 `signal.NotifyContext` 包装为等待终止信号的运行 context。启动阶段使用该信号感知 context 创建 Fx app start context；停止阶段当前改用 `context.Background()` 创建 Fx app stop context。

这种停止策略能避免 stop hooks 继承已取消的 signal context，但也丢弃了上游 context 中的 trace、日志字段、测试标记等元数据。受影响范围集中在 `user-services/cmd` 的 CLI 生命周期边界，不涉及 controller/service/repository 分层、HTTP 路由、响应契约、Redis/PostgreSQL/Ent 初始化或数据库迁移。

## Goals / Non-Goals

**Goals:**

- 让 Fx app stop context 保留 `runServe` 入参 context 中可传播的元数据。
- 保持停止阶段拥有独立的 `fxAppStopTimeout` 预算。
- 避免因终止信号导致的运行 context 取消状态直接传入 `app.Stop`。
- 用测试覆盖 stop context 的元数据传播和取消隔离。

**Non-Goals:**

- 不改变 CLI 命令名、`--config` 参数或默认配置路径。
- 不改变 HTTP server graceful shutdown 的配置值、默认值或 hook 行为。
- 不新增 common 共享能力、外部依赖、数据库 schema 或 Atlas migration。
- 不改变 HTTP API、响应信封、错误码、认证边界或路由注册顺序。

## Decisions

- 停止 root context 使用原始上游 context 的不可取消派生，而不是 `context.Background()`。
  - 理由：原始上游 context 承载调用方注入的元数据；不可取消派生可以保留这些值，同时不继承上游或 signal context 的取消状态。
  - 替代方案：继续使用 `context.Background()`。该方案简单但会继续丢弃上游元数据。
  - 替代方案：直接使用 signal-wrapped `ctx`。该方案能保留元数据，但收到终止信号后 `ctx` 已取消，会让 stop hooks 立即观察到取消状态，可能提前截断清理逻辑。

- `fxAppStopTimeout` 继续通过 `context.WithTimeout` 表达外层 Fx app 停止预算。
  - 理由：既有主规格已要求 CLI/Fx stop budget 与 HTTP graceful shutdown timeout 语义独立，且默认停止预算需要覆盖默认 HTTP shutdown 配置。
  - 替代方案：复用 HTTP shutdown timeout。该方案会混淆 CLI/Fx app 停止预算和 HTTP server graceful shutdown 预算，不符合当前 `http-service-runtime` 契约。

- 测试优先覆盖 `runServe` 的可观察生命周期行为，而非改动 bootstrap 组合根。
  - 理由：问题发生在 CLI 生命周期 context 创建策略；测试应验证 `app.Stop` 接收的 context 特性。若现有 `runServe` 难以注入测试 app，可在 `cmd` 包内引入最小的 app factory seam，保持生产路径仍调用 `bootstrap.NewApp`。
  - 替代方案：通过完整 Fx app 和真实服务依赖做集成测试。该方案成本高、依赖 Redis/PostgreSQL/配置，且不能直接稳定断言 stop context 元数据。

## Risks / Trade-offs

- [Risk] `context.WithoutCancel` 要求当前 Go 版本支持。 → 当前仓库基线为 Go 1.26，满足该 API；无需兼容更早 Go 版本。
- [Risk] 如果上游 context 自身携带 deadline，停止阶段不可取消派生会忽略该 deadline。 → 停止阶段继续由 `fxAppStopTimeout` 控制，这是本变更对“避免外部取消提前截断停止流程”的明确取舍。
- [Risk] 为测试引入 app factory seam 可能增加 `cmd` 包复杂度。 → 仅保留包内最小接口和默认工厂，避免外泄到其他模块或 common。

## Migration Plan

- 修改 `user-services/cmd/main.go` 的 stop context 创建逻辑。
- 增加或调整 `user-services/cmd` 包内测试。
- 运行 `go test ./...` 于 `user-services/`，必要时运行 `common/` 测试确认工作区未受影响。
- 回滚时可恢复为 `context.Background()` 停止 root，但会重新出现上游元数据丢失问题。

## Open Questions

- 无。
