## ADDED Requirements

### Requirement: runtime observability 测试断言迁移

`common/runtime` 中覆盖 metrics、tracing、logger、localcache、scheduler、workerpool、resources、datastore、rediskey、timezone、id 和 config 的测试 MUST 遵循统一断言规范。断言迁移 MUST 保持 Prometheus metric family、label key、label value、数值语义、tracing provider、span context、logger 字段、scheduler 行为、workerpool 行为和 runtime primitive 生产行为不变。

#### Scenario: metrics 结构化断言

- **WHEN** runtime metrics、localcache collector、scheduler metrics、workerpool metrics 或 Redis metrics 测试验证 metric family、label 或 sample 值
- **THEN** 测试 MUST 使用 `require` 或在多字段独立诊断中使用 `assert` 表达结构化断言
- **AND** 迁移 MUST NOT 通过文本格式兼容、旧 metric name、旧 label 或生产代码分支改变观测契约

#### Scenario: tracing 和 logger 断言

- **WHEN** tracing、span error、trace context、logger 或日志字段测试验证运行时输出
- **THEN** 测试 MUST 使用语义化断言表达字段存在性、字段缺失、错误状态和值匹配
- **AND** 迁移 MUST NOT 改变日志 message、稳定英文 `snake_case` 字段名、敏感信息过滤或 tracing 上下文传播语义

#### Scenario: runtime primitive 行为不变

- **WHEN** config、datastore、id、rediskey、resources、scheduler、timezone、workerpool 或 localcache 测试迁移断言风格
- **THEN** 生产 API、错误语义、生命周期、并发控制、panic recovery、shutdown 行为和配置校验结果 MUST 保持不变

#### Scenario: 并发和 panic 测试例外

- **WHEN** scheduler、workerpool、localcache 或 recovery 相关测试需要表达 goroutine 协调、panic/recovery 或 benchmark 边界
- **THEN** 测试 MAY 保留符合 `docs/TESTING.md` 例外规则的 `t.Fatal`、`t.Error` 或 `Fail*` 控制流
- **AND** 常见业务断言仍 MUST 优先迁移到 `require` 或 `assert`
