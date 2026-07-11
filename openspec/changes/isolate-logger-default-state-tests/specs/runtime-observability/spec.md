## ADDED Requirements

### Requirement: Logger 默认值测试隔离

系统 MUST 将 `common/runtime/logger` 中修改进程级默认 logger 的测试限定为验证默认 logger 兜底行为的用例。其他日志字段、trace/span 关联、SQL logger 或日志捕获测试 MUST 优先使用 context logger 或局部 logger 注入，并 MUST 保持生产日志字段、message、level 和 tracing 传播语义不变。

#### Scenario: 非默认 logger 行为测试使用局部 logger

- **WHEN** 测试验证 trace/span 字段、SQL logger、日志 message 或日志捕获结果且不需要覆盖进程级兜底 logger
- **THEN** 测试 MUST 通过 `logger.ToContext`、`logger.WithContext` 或显式传入的局部 logger 捕获日志
- **AND** 测试 MUST NOT 调用 `logger.SetDefault` 替换进程级默认 logger

#### Scenario: 默认 logger 行为测试恢复进程状态

- **WHEN** 测试必须调用 `logger.SetDefault` 验证 `FromContext` 的默认 logger 兜底行为
- **THEN** 测试 MUST 保存调用前的默认 logger 并在 cleanup 中恢复
- **AND** 该测试 MUST NOT 标记为并行测试

#### Scenario: 生产观测契约保持不变

- **WHEN** logger 默认值测试隔离完成
- **THEN** `FromContext`、`WithContext`、`SQL` 和 `SetDefault` 的生产行为 MUST 保持不变
- **AND** `trace_id`、`span_id`、logger name、日志 level 和 log message MUST 保持不变
