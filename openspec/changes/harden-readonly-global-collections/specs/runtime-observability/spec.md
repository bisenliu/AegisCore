## ADDED Requirements

### Requirement: 观测只读集合不得暴露共享可写状态
runtime observability 中用于 Prometheus label key、HTTP metrics label name 和 scheduler histogram bucket 的只读集合 MUST 使用不暴露共享可写底层状态的表达方式。实现 MUST 保持 metric family、label key、label value、label 顺序、bucket 数值和采集语义不变。

#### Scenario: 低基数 label key allowlist 不可被包内误写
- **WHEN** `common/runtime/observability/metrics` 校验 low-cardinality label key
- **THEN** allowlist MUST 使用 `switch`、私有查询函数或等价不可共享写入的表达方式
- **AND** 合法 label key、非法 label key 和校验错误语义 MUST 保持不变

#### Scenario: HTTP metrics label names 保持顺序且不可共享写入
- **WHEN** `common/http/middleware` 创建 HTTP server metrics counter、histogram 或 gauge descriptor
- **THEN** descriptor 使用的 label names MUST 保持当前顺序和名称
- **AND** 实现 MUST NOT 将可被同包未来代码修改的 package-level slice 底层数组作为 descriptor label names 的共享来源

#### Scenario: scheduler duration buckets 保持数值且不可共享写入
- **WHEN** scheduler metrics 使用默认 duration histogram buckets
- **THEN** bucket 数值和顺序 MUST 保持当前语义
- **AND** metrics 构造 MUST 不依赖可被同包未来代码修改的 package-level slice 底层数组作为共享来源

#### Scenario: 观测契约保持不变
- **WHEN** 只读集合表达被加固后导出 runtime 或 HTTP metrics
- **THEN** Prometheus metric family、label key、label value、低基数约束和数值语义 MUST 保持不变
- **AND** 系统 MUST NOT 修改 tracing、logging、request ID、pprof、OpenAPI 路由或部署观测资产
