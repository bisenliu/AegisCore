## Why

`http-service-runtime` 当前在 `runServe` 停止阶段使用 `context.Background()` 创建 Fx app stop context，导致上游传入的 trace、deadline、日志字段或测试注入的 context 元数据无法延续到 stop hooks。该问题会让外部嵌入运行命令时的生命周期控制链路不完整，也让停止阶段的设计取舍缺少明确契约。

## What Changes

- 调整 `user-services/cmd/main.go` 中 `runServe` 的停止阶段 context 策略，使 Fx app stop context 能保留上游运行 context 中可继续传播的元数据。
- 明确停止阶段必须避免直接继承已经因信号取消的 wait context，否则 stop hooks 会立即收到已取消 context。
- 增加覆盖停止 context 元数据传播和取消隔离的测试。
- 不改变 CLI 名称、配置参数、HTTP API、错误码、数据模型、Redis/PostgreSQL/Ent 运行时依赖或 HTTP graceful shutdown 超时语义。

## Capabilities

### New Capabilities


### Modified Capabilities

- `http-service-runtime`: 补充 CLI/Fx app 停止阶段 context 创建规则，要求停止 context 保留上游元数据，同时避免被终止信号产生的取消状态提前截断。

## Impact

- 影响代码：`user-services/cmd/main.go`，以及必要的同包测试。
- 影响能力：`docs/opsx/CAPABILITY_MAP.md` 中的 `http-service-runtime`。
- 外部兼容性：不改变 HTTP API、响应信封、配置格式、CLI 参数或数据库 schema。
- 运行时影响：Fx app stop hooks 可观察到从命令上游传入的 context 元数据，但仍拥有独立的停止超时预算。
