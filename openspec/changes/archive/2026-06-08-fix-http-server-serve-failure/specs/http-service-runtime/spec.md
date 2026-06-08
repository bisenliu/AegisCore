## ADDED Requirements

### Requirement: Trigger application shutdown on unexpected HTTP serve failure

HTTP 服务运行时 MUST 在 HTTP listener 成功绑定并异步执行 `Serve` 后，检测 HTTP server 的非预期退出。若 `Serve` 返回的错误不是 `http.ErrServerClosed`，系统 MUST 记录失败并触发 Fx 应用级 shutdown，使进程进入现有停止流程。正常 graceful shutdown 导致的 `http.ErrServerClosed` MUST 继续被视为预期结果，不得记录为服务失败，也不得作为新的失败触发额外处理。

#### Scenario: Unexpected serve failure triggers application shutdown
- **Given** HTTP listener 已成功绑定且 `OnStart` 已返回成功
- **When** HTTP server 的异步 `Serve` 返回非 `http.ErrServerClosed` 错误
- **Then** 系统 MUST 记录 HTTP server 失败日志
- **Then** 系统 MUST 触发 Fx 应用级 shutdown
- **Then** 服务 MUST NOT 只依赖日志表示 HTTP server 已不可用

#### Scenario: Normal server shutdown remains non-failing
- **Given** HTTP server 已成功启动
- **When** Fx app 停止并导致 `Serve` 返回 `http.ErrServerClosed`
- **Then** 系统 MUST NOT 将该错误记录为 HTTP server 失败
- **Then** 系统 MUST NOT 因该错误再次触发应用级 shutdown

#### Scenario: Startup listen failure still fails OnStart
- **Given** 配置中的 HTTP host 和 port 不可监听
- **When** Fx app 启动 HTTP server lifecycle
- **Then** `OnStart` MUST 返回包含监听失败上下文的错误
- **Then** 系统 MUST NOT 进入异步 serve 成功运行状态
