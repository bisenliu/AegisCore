## ADDED Requirements

### Requirement: Request ID 日志上下文 API 归属

系统 MUST 由 `common/runtime/logger` 统一拥有 `RequestIDField`、`WithRequestID` 和 `RequestIDFromContext`，并由 HTTP Request ID middleware 使用这些 API 将最终 request ID 写入请求 context。`common/http/middleware` MUST NOT 保留同名常量、context key、公开转发函数、别名或 deprecated wrapper。

#### Scenario: HTTP middleware 写入 logger request ID context

- **WHEN** HTTP Request ID middleware 完成入站 `X-Request-ID` 校验、透传或生成
- **THEN** middleware MUST 使用 `logger.WithRequestID` 将最终 request ID 写入 `c.Request.Context()`
- **AND** `logger.RequestIDFromContext` MUST 能读取同一个非空值

#### Scenario: 旧 middleware API 被移除

- **WHEN** 仓库编译 common 与 user-service
- **THEN** 生产代码和测试 MUST NOT 定义或引用 `middleware.RequestIDField`、`middleware.WithRequestID` 或 `middleware.RequestIDFromContext`
- **AND** 系统 MUST NOT 提供指向 logger 新 API 的兼容别名、转发函数或 deprecated wrapper

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

#### Scenario: skip endpoint 保持 in-flight gauge 正确归零

- **WHEN** HTTP metrics middleware 对 runtime endpoint 或其他被配置跳过的请求应用 skip 规则
- **THEN** 请求总数和耗时指标 MAY 不记录该请求
- **AND** in-flight gauge MUST 在请求结束时正确递减到 `0`
- **AND** 系统 MUST NOT 删除该 route label value 导致并发请求计数丢失或 gauge 状态被破坏

#### Scenario: localcache metrics 结构化输出

- **WHEN** localcache collector 导出命中、未命中、加载、singleflight、写入、驱逐和容量指标
- **THEN** 指标 MUST 使用稳定的 Prometheus metric family 表达固定 cache 名称、`result` 或 `event` label 和对应数值，且 MUST NOT 依赖文本格式解析才能验证这些结构化字段

#### Scenario: tracing provider 初始化

- **WHEN** tracing 配置启用
- **THEN** 系统 MUST 初始化 OpenTelemetry provider，并使用服务名和环境标签关联 trace

#### Scenario: trace 与 request ID 上下文传播

- **WHEN** HTTP 请求携带 W3C `traceparent` 或 `tracestate`，并且系统生成或透传 request ID
- **THEN** 系统 MUST 使用 OpenTelemetry 上下文传播
- **AND** 日志 helper MUST 只从有效 span context 派生 `trace_id` 和 `span_id`，无有效 span context 时 MUST 省略这两个字段
- **AND** 日志 helper MUST 独立从 logger request ID context 派生 `request_id`，不得因 span context 无效而省略有效 request ID

### Requirement: HTTP 请求 ID 关联

系统 MUST 为 HTTP 请求提供可由调用方观察的 request ID 关联能力。入站请求携带合法 `X-Request-ID` 时系统 MUST 透传该值；缺失或不合法时系统 MUST 生成新的 request ID。最终 request ID MUST 写入响应头 `X-Request-ID`，并 MUST 由通用 logger context 以 `request_id` 字段关联 HTTP access log 和请求生命周期内通过共享 logger 记录的应用日志。

#### Scenario: 透传入站请求 ID

- **WHEN** HTTP 请求携带合法 `X-Request-ID`
- **THEN** 系统 MUST 在响应头 `X-Request-ID` 中回传相同值
- **AND** HTTP access log 和请求生命周期内的共享 logger 日志 MUST 使用 `request_id` 字段记录相同值

#### Scenario: 生成缺失请求 ID

- **WHEN** HTTP 请求未携带 `X-Request-ID`
- **THEN** 系统 MUST 生成新的 request ID
- **AND** 响应头 `X-Request-ID`、HTTP access log 和请求生命周期内的共享 logger 日志 MUST 使用该生成值

#### Scenario: 拒绝不合法请求 ID

- **WHEN** HTTP 请求携带空白、超长或包含控制字符的 `X-Request-ID`
- **THEN** 系统 MUST 不透传该不合法值
- **AND** 系统 MUST 生成新的 request ID 并写入响应头、HTTP access log 和请求生命周期内的共享 logger 日志

#### Scenario: request ID 与 tracing 并存

- **WHEN** HTTP 请求携带 W3C `traceparent` 且系统生成或透传 `X-Request-ID`
- **THEN** HTTP access log 和请求生命周期内的共享 logger 日志 MUST 在 span context 有效时同时包含 `trace_id`、`span_id` 和 `request_id`
- **AND** span context 无效时日志 MUST 省略 `trace_id` 和 `span_id`，但 MUST 保留有效 `request_id`
- **AND** request ID 行为 MUST NOT 改变现有 `traceparent` 或 `tracestate` 传播语义

#### Scenario: 参数校验失败日志关联 request ID

- **WHEN** `BindOrAbort` 因请求绑定或字段校验失败记录 `invalid request` 应用日志
- **THEN** 日志 MUST 自动包含当前请求的 `request_id`
- **AND** binding 层 MUST NOT 手工读取或重复追加 request ID 字段

#### Scenario: access log request ID 字段唯一

- **WHEN** HTTP request logger 通过 `logger.WithContext` 记录请求完成日志
- **THEN** access log MUST 仅包含一个 `request_id` 字段
- **AND** access log 专用字段构造 MUST NOT 再次手工追加 request ID

#### Scenario: metrics 标签不包含请求 ID

- **WHEN** 系统记录 HTTP 或 runtime metrics 标签
- **THEN** metrics 标签 MUST NOT 包含 `request_id`、`X-Request-ID` 或任何等价高基数请求标识

### Requirement: Logger 默认值测试隔离

系统 MUST 将 `common/runtime/logger` 中修改进程级默认 logger 的测试限定为验证默认 logger 兜底行为的用例。其他日志字段、trace/span/request ID 关联、SQL logger 或日志捕获测试 MUST 优先使用 context logger 或局部 logger 注入，并 MUST 保持生产日志字段、message、level 和 tracing 传播语义不发生本变更未声明的变化。

#### Scenario: 非默认 logger 行为测试使用局部 logger

- **WHEN** 测试验证 trace/span/request ID 字段、SQL logger、日志 message 或日志捕获结果且不需要覆盖进程级兜底 logger
- **THEN** 测试 MUST 通过 `logger.ToContext`、`logger.WithContext`、`logger.WithRequestID` 或显式传入的局部 logger 捕获日志
- **AND** 测试 MUST NOT 调用 `logger.SetDefault` 替换进程级默认 logger

#### Scenario: 默认 logger 行为测试恢复进程状态

- **WHEN** 测试必须调用 `logger.SetDefault` 验证 `FromContext` 的默认 logger 兜底行为
- **THEN** 测试 MUST 保存调用前的默认 logger 并在 cleanup 中恢复
- **AND** 该测试 MUST NOT 标记为并行测试

#### Scenario: 生产观测契约按声明扩展

- **WHEN** logger request ID 上下文能力完成迁移
- **THEN** `FromContext` 和 `WithContext` MUST 在 request ID context 有效时附加 `request_id`
- **AND** `SQL`、`SetDefault`、`trace_id`、`span_id`、logger name、日志 level 和 log message 的既有行为 MUST 保持不变
