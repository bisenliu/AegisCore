## ADDED Requirements

### Requirement: Auth 资源停止测试具备硬超时

auth session purge pool、token-version 本地缓存和 auth module 关闭顺序测试 MUST 保留 Fx event 诊断信息，并对 worker pool stop、启动失败回滚和共享 Redis 关闭顺序验证提供测试级硬超时保护。

#### Scenario: session purge pool stop timeout 不阻塞测试

- **WHEN** 测试 session purge pool 的 drain、caller timeout 或重复停止行为
- **THEN** 测试 MUST 使用带 timeout 的 context 和测试级 guard 等待 Stop 返回
- **AND** worker task 忽略 context 或 stop 实现阻塞时测试 MUST 在测试级 guard 内失败

#### Scenario: auth module 关闭顺序可诊断

- **WHEN** 测试 auth module 停止 auth 自有资源早于共享 Redis client
- **THEN** 测试 MUST 使用 `fxtest.New(t, ...)` 默认测试 logger 或 `fxtest.WithTestLogger(t)`
- **AND** 测试 MUST NOT 使用 `fx.NopLogger` 静默 Fx lifecycle event

#### Scenario: auth 启动失败回滚停止自有资源

- **WHEN** auth module 后续 `OnStart` hook 失败
- **THEN** 测试 MUST 验证 session purge pool 和 token-version 本地缓存按已启动资源回滚关闭
- **AND** Start 调用 MUST 使用带 deadline 的 context 或测试级 guard，避免回滚阻塞导致测试无限等待
