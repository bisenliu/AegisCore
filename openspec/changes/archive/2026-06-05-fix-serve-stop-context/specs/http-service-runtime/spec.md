## ADDED Requirements

### Requirement: Preserve upstream context metadata during CLI stop

用户服务 CLI 停止 Fx app 时，系统 MUST 使用从 `runServe` 上游 context 派生的 stop root context，以保留调用方注入的 context values。该 stop root context MUST NOT 直接继承终止信号触发后的取消状态；Fx app stop context MUST 继续使用 `fxAppStopTimeout` 表达独立停止预算。

#### Scenario: Stop context preserves upstream values
- **Given** 调用方使用携带 context value 的上游 context 调用 `runServe`
- **When** CLI 收到 `os.Interrupt` 或 `SIGTERM` 并触发 Fx app stop
- **Then** Fx app stop hooks MUST 能通过 stop context 读取该上游 context value
- **Then** stop context MUST 继续使用 CLI/Fx app stop timeout 作为停止预算

#### Scenario: Stop context does not inherit signal cancellation
- **Given** 服务运行 context 因 `os.Interrupt` 或 `SIGTERM` 变为已取消
- **When** CLI 创建传给 Fx app stop hooks 的 stop context
- **Then** stop context MUST NOT 因该终止信号而已经处于取消状态
- **Then** stop hooks MUST 仍可在 `fxAppStopTimeout` 预算内执行清理逻辑

#### Scenario: Runtime surface remains unchanged
- **Given** CLI stop context 创建策略已调整
- **When** 用户服务通过 `aegiscore-user-services serve` 启动并停止
- **Then** CLI 命令名、`--config` 参数、HTTP 路由、响应信封、认证边界和 runtime 依赖初始化 MUST 保持不变
- **Then** HTTP server graceful shutdown MUST 继续使用现有配置和默认 timeout 规则
