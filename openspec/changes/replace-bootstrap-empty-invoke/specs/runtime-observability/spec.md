## MODIFIED Requirements

### Requirement: 运行时故障、初始化保护与优雅关闭

系统 MUST 将 HTTP 或 pprof listener 的非预期退出转换为 Fx shutdown signal，并在统一的 `runtime.lifecycle.stop_timeout` 总预算内按逆序 lifecycle hook 完成优雅关闭。可预期的资源、配置和依赖错误 MUST 优先通过 constructor 返回 `error` 暴露，MUST NOT 依赖 panic recovery 表达正常失败路径。正式 App 的 composition root MUST 以清晰、可验证的方式解析 HTTP server 与 pprof server，使对应 constructor 注册 lifecycle hook。

#### Scenario: listener 非预期退出

- **WHEN** HTTP 或 pprof `Serve` 在未进入正常关闭阶段时返回错误
- **THEN** 系统 MUST 记录可诊断错误并触发非零内部 shutdown signal
- **WHEN** 正常关闭导致 `http.ErrServerClosed`
- **THEN** 系统 MUST NOT 将其视为内部故障

#### Scenario: 外部与内部退出共用预算

- **WHEN** 外部终止信号或内部故障触发关闭
- **THEN** 系统 MUST 使用同一未被取消的上游 context value 和 `runtime.lifecycle.stop_timeout` 总预算执行 `App.Stop`
- **AND** 局部 HTTP、gRPC、tracing 或 logger timeout MUST NOT 替代总预算
- **WHEN** 前序 `OnStop` hook 已消耗部分总预算
- **THEN** 后续 hook MUST 只使用剩余时间，总关闭耗时 MUST NOT 因每个组件重新创建完整预算而无界增长

#### Scenario: 快速正常关闭和 pprof 强制关闭

- **WHEN** 所有 hook 在预算内完成
- **THEN** App MUST 立即完成关闭，不得等待完整 timeout
- **WHEN** pprof 已启用且 `OnStop` 调用 `server.Shutdown(ctx)` 返回错误
- **THEN** 系统 MUST 对同一个 pprof server 执行 best-effort `server.Close()`
- **AND** 返回错误 MUST 保留 `Shutdown` 失败信息，当 `Close` 也失败时 MUST 同时包含强制关闭失败信息
- **AND** 重复停止 MUST NOT panic 或阻塞

#### Scenario: Fx DI 初始化边界保护

- **WHEN** Fx 在 user-service composition root 中执行 constructor、decorator 或 Invoke 时发生未预期 panic
- **THEN** App 构造或启动 MUST 通过 Fx error 暴露 panic 信息
- **AND** 进程 MUST NOT 因该 DI 初始化 panic 直接崩溃
- **WHEN** HTTP handler、worker task、后台 goroutine 或 lifecycle hook 运行期发生 panic
- **THEN** `fx.RecoverFromPanics()` MUST NOT 被视为这些运行期边界的恢复策略
- **AND** 对应边界 MUST 使用其自身已有或显式设计的 panic 处理机制

#### Scenario: runtime server lifecycle 注册可验证

- **WHEN** user-service 通过正式 `AppModule` 构建 runtime graph
- **THEN** composition root MUST 显式解析 `*http.Server` 与 `*PprofServer`
- **AND** 解析意图 MUST 通过具名注册函数或等价可识别结构表达，MUST NOT 依赖空匿名 Invoke 隐式表达
- **AND** bootstrap 测试 MUST 能验证正式 runtime graph 仍解析这些 server 并保留 lifecycle hook 注册链路
