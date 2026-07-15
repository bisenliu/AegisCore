## ADDED Requirements

### Requirement: Logger 构造函数无进程默认副作用

`common/runtime/logger` 的 logger 构造函数 MUST 只构造并返回调用方拥有的 `*zap.Logger`，不得隐式安装、覆盖或恢复进程级默认 logger。进程级默认 logger 只作为共享 helper 的无注入兜底能力存在，调用方需要修改该兜底值时 MUST 显式调用 `SetDefault`。

#### Scenario: New 不覆盖默认 logger
- **WHEN** 调用方执行 `logger.New` 创建配置化 logger
- **THEN** 系统 MUST 返回该 logger
- **AND** 进程级默认 logger MUST 保持调用前的值

#### Scenario: NewWithConfig 不覆盖默认 logger
- **WHEN** 调用方执行 `logger.NewWithConfig` 创建配置化 logger
- **THEN** 系统 MUST 返回该 logger
- **AND** 进程级默认 logger MUST 保持调用前的值

#### Scenario: NewLogger 不覆盖默认 logger
- **WHEN** Fx provider `logger.NewLogger` 创建正式 logger
- **THEN** 系统 MUST 返回该 logger 并注册既有 Sync 关闭 hook
- **AND** 系统 MUST NOT 调用 `SetDefault` 或等价逻辑覆盖进程级默认 logger

#### Scenario: 显式默认 logger API 保留兜底语义
- **WHEN** 共享 helper 在没有注入 logger 或 context logger 的情况下调用 `FromContext`、`WithContext`、`NamedComponent` 或包级日志函数
- **THEN** 系统 MAY 使用当前进程级默认 logger 作为兜底
- **AND** 该默认值只能通过显式 `SetDefault` 修改
- **AND** logger 构造函数 MUST NOT 成为修改默认值的隐式入口
