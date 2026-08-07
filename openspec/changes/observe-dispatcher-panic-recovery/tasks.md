## 1. OpenSpec

- [x] 1.1 为 `rbac-access-control` 创建 dispatcher panic recovery 可观测性 spec delta。
- [x] 1.2 明确不引入自动重启，不改变 outbox retry、claim lease 或投递状态机。

## 2. 实现

- [x] 2.1 在 `Dispatcher.run` defer recovery 中记录 `error_category=unexpected_exit`。
- [x] 2.2 在同一条 recovery error 日志中记录 `recovered` 字段。
- [x] 2.3 在同一条 recovery error 日志中记录 stack trace。
- [x] 2.4 保持 panic 后 ticker stop、`running=false`、当前 `done` close 和 `DispatcherRunningObserved(false)` 行为不变。

## 3. 测试与验证

- [x] 3.1 更新 dispatcher unexpected exit 测试，断言状态与 Stop 幂等。
- [x] 3.2 增加日志字段断言，覆盖 `error_category`、`recovered` 和 stack trace。
- [x] 3.3 运行 `openspec validate observe-dispatcher-panic-recovery --strict`。
- [x] 3.4 运行 `go test ./user-service/internal/features/permission/application -run 'TestDispatcherUnexpectedExit|TestDispatcherStartStop'`。
