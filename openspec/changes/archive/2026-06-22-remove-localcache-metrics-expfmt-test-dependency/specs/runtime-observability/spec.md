## MODIFIED Requirements

### Requirement: Metrics 和 tracing

系统 MUST 提供 Prometheus metrics 与 OpenTelemetry tracing 基础能力，并通过共享 provider 保持服务、环境和资源标签一致。runtime metrics 中的 localcache 指标 MUST 保持稳定的指标名称、label key、label value 和数值语义，使测试和观测消费方能够按结构化 metric family 验证 `cache`、`result`、`event` 等低基数标签。

#### Scenario: 访问 metrics

- **WHEN** metrics 配置允许暴露指标
- **THEN** user-service MUST 在 `/api/v1` 外注册配置化 metrics 路由，并导出 HTTP、SQL、Redis、runtime、scheduler、workerpool 或 localcache 相关指标；metrics 路由 MUST NOT 经过 RBAC 授权

#### Scenario: metrics 配置禁用

- **WHEN** metrics 暴露被配置为禁用
- **THEN** 系统 MUST 不暴露 metrics 路由或返回符合配置的禁用行为

#### Scenario: metrics 标签

- **WHEN** 系统记录 metrics 标签
- **THEN** 标签 MUST 保持低基数，MUST NOT 包含用户 ID、角色 ID、权限 ID、会话 ID、token ID、trace/span ID、raw path、IP、邮箱、用户名、SQL、Redis key 或原始错误

#### Scenario: localcache metrics 结构化输出

- **WHEN** localcache collector 导出命中、未命中、加载、singleflight、写入、驱逐和容量指标
- **THEN** 指标 MUST 使用稳定的 Prometheus metric family 表达固定 cache 名称、`result` 或 `event` label 和对应数值，且 MUST NOT 依赖文本格式解析才能验证这些结构化字段

#### Scenario: tracing provider 初始化

- **WHEN** tracing 配置启用
- **THEN** 系统 MUST 初始化 OpenTelemetry provider，并使用服务名和环境标签关联 trace

#### Scenario: trace 上下文传播

- **WHEN** HTTP 请求携带 W3C `traceparent` 或 `tracestate`
- **THEN** 系统 MUST 使用 OpenTelemetry 上下文传播；日志 helper MUST 只从有效 span context 派生 `trace_id` 和 `span_id`，无有效 span context 时 MUST 省略这些字段
