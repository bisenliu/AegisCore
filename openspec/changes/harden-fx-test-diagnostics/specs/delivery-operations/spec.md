## ADDED Requirements

### Requirement: Fx 测试诊断与硬超时门禁

Go 测试 MUST 保留 Fx event 的测试诊断信息，并且对可能阻塞的启动、停止、rollback、worker 或 shutdown 测试路径提供测试级硬超时保护。除非测试明确断言构图或启动错误且 Fx event 日志确实无价值，测试 MUST NOT 使用 `fx.NopLogger` 静默 Fx event。

#### Scenario: Fx 正向组合测试输出 event

- **WHEN** 测试使用 `fxtest.New(t, ...)` 构造并启动正向 Fx app 或 feature module
- **THEN** 测试 MUST 使用 `fxtest.New` 默认测试 logger 或显式 `fxtest.WithTestLogger(t)`
- **AND** 测试 MUST NOT 额外传入 `fx.NopLogger` 覆盖测试 logger

#### Scenario: Fx 负向构图测试保留可诊断日志

- **WHEN** 测试需要使用 `fx.New` 并断言 `app.Err()`
- **THEN** 测试 MUST 显式配置 `fxtest.WithTestLogger(t)` 或等价测试 logger
- **AND** 测试 MUST NOT 改用会在 `app.Err()` 非空时提前 `FailNow` 的 `fxtest.New`

#### Scenario: 生命周期 spy 启用硬超时

- **WHEN** 测试使用 `fxtest.NewLifecycle` 验证可能阻塞或依赖 context deadline 的 `OnStart` 或 `OnStop` hook
- **THEN** 测试 MUST 启用 `fxtest.EnforceTimeout(true)`
- **AND** hook 忽略 context 时测试 MUST 在自身 deadline 到期后返回 context 相关错误，而不是等待全局 `go test -timeout`

#### Scenario: 直接 Stop 调用具备测试 guard

- **WHEN** 测试直接调用 `Stop(ctx)`、`Shutdown(ctx)`、关闭 hook 或其它可能阻塞的函数
- **THEN** 测试 MUST 使用带 timeout 的 context 并通过 goroutine/select、`require.Eventually` 或等价机制限制测试等待时间
- **AND** 被测实现不尊重 context 时测试 MUST 在测试级 guard 内失败
