## MODIFIED Requirements

### Requirement: 监听故障与优雅关闭

系统 MUST 将 HTTP 或 pprof listener 的非预期退出转换为 Fx shutdown signal，并在统一的 `runtime.lifecycle.stop_timeout` 总预算内按逆序 lifecycle hook 完成优雅关闭。pprof listener 在 graceful shutdown 失败时 MUST 执行 best-effort 强制关闭，避免 listener 或 `Serve` goroutine 滞留至进程退出。

#### Scenario: listener 非预期退出

- **WHEN** HTTP 或 pprof `Serve` 在未进入正常关闭阶段时返回错误
- **THEN** 系统 MUST 记录可诊断错误并触发非零内部 shutdown signal
- **WHEN** 正常关闭导致 `http.ErrServerClosed`
- **THEN** 系统 MUST NOT 将其视为内部故障

#### Scenario: 外部与内部退出共用预算

- **WHEN** 外部终止信号或内部故障触发关闭
- **THEN** 系统 MUST 使用同一未被取消的上游 context value 和 `runtime.lifecycle.stop_timeout` 总预算执行 `App.Stop`
- **AND** 局部 HTTP、gRPC、tracing 或 logger timeout MUST NOT 替代总预算

#### Scenario: 前序 hook 消耗时间

- **WHEN** 前序 `OnStop` hook 已消耗部分总预算
- **THEN** 后续 hook MUST 只使用剩余时间
- **AND** 总关闭耗时 MUST NOT 因每个组件重新创建完整预算而无界增长

#### Scenario: lifecycle timeout 同源

- **WHEN** App 和 CLI 构建启动或停止 context
- **THEN** 两者 MUST 使用同一已加载并校验的 lifecycle 配置
- **AND** `fx.New` 构造期 MUST NOT 被误算入 `StartTimeout`，也 MUST NOT 为满足 timeout 而隐式迁移现有资源构造语义

#### Scenario: 快速正常关闭

- **WHEN** 所有 hook 在预算内完成
- **THEN** App MUST 立即完成关闭，不得等待完整 timeout

#### Scenario: pprof graceful shutdown 失败后强制关闭

- **WHEN** pprof 已启用且 `OnStop` 调用 `server.Shutdown(ctx)` 返回错误
- **THEN** 系统 MUST 对同一个 pprof server 执行 best-effort `server.Close()`
- **AND** 返回错误 MUST 保留 `Shutdown` 失败信息
- **AND** 当 `Close` 也失败时，返回错误 MUST 同时包含强制关闭失败信息
- **AND** `Serve` goroutine MUST 因 listener 关闭退出，且正常关闭产生的 `http.ErrServerClosed` MUST NOT 触发非零内部 shutdown signal

#### Scenario: pprof 停止幂等性

- **WHEN** pprof server 已经被关闭后再次进入停止路径
- **THEN** 系统 MUST NOT 因重复 `Close` 导致 panic 或阻塞
- **AND** 重复停止产生的关闭错误 MUST 作为诊断返回或被原有 `http.Server` 语义吸收
