## Why

当前 RBAC outbox dispatcher 后台 goroutine 的 panic recovery 只记录 `error_category=unexpected_exit`，没有记录 panic 的 recovered value，也没有稳定暴露 stack trace。dispatcher panic 后虽然会 fail-closed 并更新状态，但排障日志缺少定位 panic 来源的关键上下文。

## What Changes

- 增强 `Dispatcher.run` 的 defer recovery 日志，记录 `error_category=unexpected_exit`、`recovered` 字段和 stack trace。
- 保持 panic 后 dispatcher 停止运行、`running=false`、`done` close、ticker stop、`DispatcherRunningObserved(false)` 和 `LastErrorCategory=unexpected_exit` 的既有行为。
- 更新 dispatcher unexpected exit 单元测试，断言 recovery 日志包含 `recovered`、`error_category` 和 stack 字段。
- 不改变 dispatcher 自动重启策略、outbox retry、claim lease、投递状态机、`Start(ctx)` 或 `DispatchOnce` 的结构化语义。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`：补充 RBAC policy outbox dispatcher 后台 panic recovery 的可观测性要求。

## Impact

- 影响代码：`user-service/internal/features/permission/application/dispatcher.go` 和 dispatcher 单元测试。
- 影响行为：dispatcher 后台 panic 时的 error 日志会包含 recovered value 和 stack trace，panic 后仍保持 fail-closed 停止状态。
- 不影响公开 HTTP API、OpenAPI、数据库 schema、migration、Redis 消息格式、outbox event payload、自动重启策略或投递状态机。
- 验证范围：`go test ./user-service/internal/features/permission/application -run 'TestDispatcherUnexpectedExit|TestDispatcherStartStop'`。
