## ADDED Requirements

### Requirement: HTTP 请求 ID 关联

系统 MUST 为 HTTP 请求提供可由调用方观察的 request ID 关联能力。入站请求携带合法 `X-Request-ID` 时系统 MUST 透传该值；缺失或不合法时系统 MUST 生成新的请求 ID。最终 request ID MUST 写入响应头 `X-Request-ID`，并 MUST 以 `request_id` 字段出现在请求日志中。

#### Scenario: 透传入站请求 ID

- **WHEN** HTTP 请求携带合法 `X-Request-ID`
- **THEN** 系统 MUST 在响应头 `X-Request-ID` 中回传相同值
- **AND** 请求日志 MUST 使用 `request_id` 字段记录相同值

#### Scenario: 生成缺失请求 ID

- **WHEN** HTTP 请求未携带 `X-Request-ID`
- **THEN** 系统 MUST 生成新的请求 ID
- **AND** 响应头 `X-Request-ID` 与请求日志字段 `request_id` MUST 使用该生成值

#### Scenario: 拒绝不合法请求 ID

- **WHEN** HTTP 请求携带空白、超长或包含控制字符的 `X-Request-ID`
- **THEN** 系统 MUST 不透传该不合法值
- **AND** 系统 MUST 生成新的请求 ID 并写入响应头和请求日志

#### Scenario: request ID 与 tracing 并存

- **WHEN** HTTP 请求携带 W3C `traceparent` 且系统生成或透传 `X-Request-ID`
- **THEN** 请求日志 MUST 在 span context 有效时同时包含 `trace_id`、`span_id` 和 `request_id`
- **AND** request ID 行为 MUST NOT 改变现有 `traceparent` 或 `tracestate` 传播语义

#### Scenario: metrics 标签不包含请求 ID

- **WHEN** 系统记录 HTTP 或 runtime metrics 标签
- **THEN** metrics 标签 MUST NOT 包含 `request_id`、`X-Request-ID` 或任何等价高基数请求标识
