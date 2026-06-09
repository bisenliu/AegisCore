## ADDED Requirements

### Requirement: Manage HTTP server goroutine with lifecycle context

HTTP 服务运行时 MUST 将 Fx `OnStart` 中启动的 HTTP server 异步 goroutine 作为 lifecycle 托管资源，并 MUST 为该 goroutine 提供可取消 context。Fx `OnStop` MUST 触发该 context 取消，并 MUST 继续执行现有 HTTP graceful shutdown。HTTP server goroutine MUST 能在 context 取消、正常 `http.ErrServerClosed` 或非预期 `Serve` 错误后退出；只有非预期 `Serve` 错误 MUST 被记录为服务失败并触发 Fx 应用级 shutdown。

#### Scenario: Stop cancels HTTP server goroutine context
- **Given** HTTP listener 已成功绑定且 `Serve` goroutine 已启动
- **When** Fx app 停止并执行 HTTP server `OnStop`
- **Then** `OnStop` MUST 取消托管该 goroutine 的 context
- **Then** HTTP server MUST 继续使用 `http.shutdown_timeout` 或默认 `10s` 执行 graceful shutdown
- **Then** goroutine MUST 能观察到生命周期取消并退出

#### Scenario: Normal server closed exit remains non-failing
- **Given** HTTP server goroutine 已由 lifecycle context 托管
- **When** graceful shutdown 导致 `Serve` 返回 `http.ErrServerClosed`
- **Then** 系统 MUST NOT 将该错误记录为 HTTP server 失败
- **Then** 系统 MUST NOT 因该错误再次触发应用级 shutdown
- **Then** goroutine 退出 MUST 不依赖无限阻塞或不可取消等待

#### Scenario: Unexpected serve failure still triggers application shutdown
- **Given** HTTP server goroutine 已由 lifecycle context 托管
- **When** `Serve` 返回非 `http.ErrServerClosed` 错误且 lifecycle context 尚未表示正常停止完成
- **Then** 系统 MUST 记录 HTTP server 失败日志
- **Then** 系统 MUST 触发 Fx 应用级 shutdown
- **Then** goroutine MUST 在错误处理完成后退出

#### Scenario: Future HTTP runtime goroutines require cancellation boundary
- **Given** 维护者在 `http-service-runtime` 的 HTTP server lifecycle 中新增异步 goroutine
- **When** 该 goroutine 执行循环、重试、阻塞上报或等待多个资源
- **Then** 该 goroutine MUST 接收可取消 context 或等价停止信号
- **Then** 该 goroutine MUST 在 lifecycle 停止时有明确退出路径
- **Then** 该 goroutine 的正常退出和异常退出 MUST 具备可测试或可观测行为
