## ADDED Requirements

### Requirement: 运行时关闭测试具备诊断与硬超时

runtime observability 相关的 HTTP drain、pprof shutdown、Fx lifecycle shared deadline 和 tracing exporter shutdown 测试 MUST 保留 Fx event 或组件日志诊断信息，并对阻塞关闭路径提供测试级硬超时保护。

#### Scenario: HTTP drain timeout 测试不无限等待

- **WHEN** 测试 HTTP server shutdown、active handler drain 或 drain tracker timeout
- **THEN** 测试 MUST 使用明确的 context deadline 和测试级等待上限
- **AND** handler、drain tracker 或 shutdown hook 忽略 context 时测试 MUST 快速失败而不是等待全局测试 timeout

#### Scenario: pprof shutdown timeout 测试不无限等待

- **WHEN** 测试 pprof server 停止、重复停止或强制关闭行为
- **THEN** 测试 MUST 对 OnStop 调用或后台请求等待设置测试级 guard
- **AND** 失败输出 MUST 保留可定位 pprof shutdown 或 listener 错误的日志信息

#### Scenario: tracing exporter start 或 shutdown 阻塞

- **WHEN** 测试 tracing exporter 创建、启动 context 或 shutdown 行为
- **THEN** 测试 MUST 使用带 timeout 的 start/stop context 或 `fxtest.EnforceTimeout(true)` 保护可阻塞 hook
- **AND** exporter 忽略 context 时测试 MUST 在测试级 guard 内失败

#### Scenario: Fx lifecycle shared deadline 可诊断

- **WHEN** 测试 Fx lifecycle stop 顺序或剩余 deadline 传播
- **THEN** 测试 MUST 使用测试 logger 或可观察断言保留 hook 执行诊断信息
- **AND** 可能阻塞的 hook 测试 MUST 启用硬超时保护
