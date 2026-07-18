## ADDED Requirements

### Requirement: 共享 tracing 与 datastore constructor 错误语义
`common/runtime/observability/tracing` 和 `common/runtime/datastore` MUST 为服务装配提供可预测的 constructor 错误语义。共享 runtime primitive MUST 将可预期的配置、资源和 instrumentation 失败返回为 error，MUST NOT 通过 panic 表达这些失败。

#### Scenario: Fx tracing provider 可直接消费
- **WHEN** 服务通过共享 Fx provider 构造 tracing provider
- **THEN** 返回的 provider MUST 立即具备可供 instrumentation 使用的非 nil tracer provider
- **AND** provider shutdown MUST 由 Fx lifecycle stop hook 管理

#### Scenario: Redis client instrumentation 错误
- **WHEN** `common/runtime/datastore` 创建 Redis client 时 tracing instrumentation 失败
- **THEN** constructor MUST 返回可匹配和可诊断的 error
- **AND** constructor MUST 关闭该 Redis client 并保留 instrumentation 失败与关闭失败信息

#### Scenario: 不为测试扩张生产 API
- **WHEN** 测试需要验证 tracing 或 datastore 失败路径
- **THEN** 测试 MUST 使用 package-local fixture、已有最小注入点或 Fx graph 断言
- **AND** 正式代码 MUST NOT 为测试新增无运行时职责的公开 wrapper、全局开关或兼容路径
