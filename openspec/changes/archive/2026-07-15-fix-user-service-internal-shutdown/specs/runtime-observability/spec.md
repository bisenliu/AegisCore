## ADDED Requirements

### Requirement: 监听服务内部故障触发失败关闭信号

user-service 的 HTTP 与 pprof listener/server 在 Fx App 成功启动后发生非预期退出时，系统 MUST 记录可诊断错误并调用 `fx.Shutdowner.Shutdown(fx.ExitCode(1))`。正常生命周期关闭产生的预期 Serve 结果 MUST NOT 触发失败 shutdown signal。

#### Scenario: HTTP Serve 非预期退出

- **WHEN** HTTP server 的 `Serve` 在生命周期未进入正常停止状态时返回非预期 listener 或服务错误
- **THEN** bootstrap MUST 调用 `Shutdown(fx.ExitCode(1))`
- **AND** 错误日志 MUST 保留 HTTP server 故障原因

#### Scenario: pprof Serve 非预期退出

- **WHEN** 已启用的独立 pprof server 因非预期 listener 关闭或服务错误退出
- **THEN** bootstrap MUST 调用 `Shutdown(fx.ExitCode(1))`
- **AND** 错误日志 MUST 保留 pprof server 故障原因

#### Scenario: 正常关闭监听服务

- **WHEN** Fx 生命周期停止 HTTP 或 pprof server，且 `Serve` 返回 `http.ErrServerClosed` 或能由生命周期取消证明的预期关闭错误
- **THEN** bootstrap MUST NOT 把该结果报告为内部故障
- **AND** bootstrap MUST NOT 因该结果额外调用失败 shutdown signal

#### Scenario: 请求关闭失败

- **WHEN** listener/server 故障发生后 `Shutdown(fx.ExitCode(1))` 返回错误
- **THEN** bootstrap MUST 记录 shutdown 请求失败及其错误原因
- **AND** bootstrap goroutine MUST NOT 直接调用 `App.Stop()` 或 `os.Exit`
