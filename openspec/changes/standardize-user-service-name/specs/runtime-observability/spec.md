## MODIFIED Requirements

### Requirement: Metrics 平台、依赖资源名与低基数

系统 MUST 提供 Prometheus metrics 基础能力，并以非 nil provider 显式表达启用或禁用状态。HTTP、runtime、scheduler、workerpool、SQL、Redis 和 feature metrics MUST 保持稳定、低基数且不泄露敏感数据。user-service 主 PostgreSQL runtime dependency 的资源名 MUST 为 `primary_db`，Redis 缓存资源名保持 `cache_redis`。user-service 默认 metrics `service` label、tracing `service.name`、日志 `service` 字段、健康响应 service 字段、dashboard 变量和 alert 表达式 MUST 统一使用 `aegiscore-user-service`；旧 `aegiscore-user-services` label 和兼容 PromQL MUST NOT 保留。

#### Scenario: metrics 启停和依赖图完整

- **WHEN** metrics 暴露被启用
- **THEN** 系统 MUST 注册配置化 metrics endpoint 并导出已注册 collector
- **WHEN** metrics 被禁用
- **THEN** 系统 MUST 不暴露 endpoint 或 collector，但 MUST 向正式依赖图提供非 nil no-op provider
- **AND** metrics/tracing 以及 feature-local `Metrics` 输入 MUST 是非 optional 的明确依赖，缺失依赖 MUST 导致构图失败

#### Scenario: 低基数标签和资源名

- **WHEN** 系统记录 metrics、健康检查或告警查询
- **THEN** label MUST NOT 包含用户、角色、权限、会话或 token ID、trace/span ID、raw path、IP、邮箱、用户名、SQL、Redis key 或原始错误
- **AND** PostgreSQL 资源名 MUST 使用 `primary_db`，Redis 缓存资源名 MUST 使用 `cache_redis`
- **AND** 指标 label、健康检查名称、dashboard 查询和 alert 表达式 MUST 保持一致
- **AND** 低基数 label allowlist、HTTP label names 和 duration buckets 的顺序与数值 MUST 保持稳定且不可被调用方修改
- **AND** user-service 默认 `service` label MUST 为 `aegiscore-user-service`

#### Scenario: HTTP in-flight gauge 正确归零

- **WHEN** metrics middleware 跳过 runtime endpoint 或其他配置化请求
- **THEN** 请求总数和耗时 MAY 不记录该请求
- **AND** in-flight gauge MUST 在请求结束后递减到 `0`，MUST NOT 因删除共享 label value 破坏并发计数

#### Scenario: feature metrics no-op 归属

- **WHEN** feature-local `Metrics` interface 需要空实现
- **THEN** 系统 MUST 通过统一生成入口维护匹配接口的 no-op 实现
- **AND** 业务指标方法 MUST 留在所属 feature，`common/runtime/observability/metrics` MUST NOT 承载 user-service 业务语义

#### Scenario: 观测资产命名一致

- **WHEN** Prometheus rules、Grafana dashboard、Compose scrape config 或观测文档引用 user-service
- **THEN** 查询、默认变量、静态 label 和告警 label MUST 使用 `aegiscore-user-service`
- **AND** dashboard UID、rule group、job name 和文档示例 MUST 与单数服务名一致
- **AND** 旧 `aegiscore-user-services` 查询、变量默认值或兼容 dashboard MUST NOT 保留

### Requirement: 结构化日志、请求关联与 Fx 事件

系统 MUST 为每个 HTTP 请求建立 request ID，并通过共享 logger context 将其与 access log、应用日志、有效 tracing context 和 Fx 初始化事件关联。日志 MUST 结构化输出到 stdout/stderr，保持分级且不得记录敏感信息。user-service 默认日志身份字段 `service` MUST 使用 `aegiscore-user-service`，并与 metrics、tracing 和健康响应中的服务名一致。

#### Scenario: request ID 和 trace 字段

- **WHEN** 入站 `X-Request-ID` 合法
- **THEN** 系统 MUST 在响应头和请求日志中透传相同值
- **WHEN** header 缺失、空白、超长或含控制字符
- **THEN** 系统 MUST 生成新值并用于响应头、access log 和应用日志
- **AND** middleware MUST 使用 `common/runtime/logger` 写入和读取最终 request ID
- **WHEN** 请求具有有效 W3C trace context
- **THEN** 日志 MUST 同时包含独立的 `request_id`、`trace_id` 和 `span_id`
- **AND** span context 无效时日志 MUST 省略 trace 字段但保留有效 request ID，metrics label MUST NOT 包含这些关联 ID

#### Scenario: 日志和 panic 可观测

- **WHEN** 请求完成、发生 panic 或 span 对应操作失败
- **THEN** access log MUST 记录稳定字段，recovery MUST 记录错误并返回统一响应，span MUST 标记错误
- **AND** 日志 MUST NOT 包含密码、token、Cookie、Authorization、DSN、SQL 参数或完整 Redis key

#### Scenario: logger 生命周期与隔离

- **WHEN** 正式 App 启停或多个 App 并行运行
- **THEN** App MUST 使用显式注入的 logger，在 Stop 时同步自身 logger，且 MUST NOT 安装或依赖进程级默认 logger
- **AND** logger 默认值相关测试 MUST 隔离并恢复进程状态

#### Scenario: Fx event 结构化日志

- **WHEN** user-service 通过 `AppOptions` 或 `NewApp` 构建正式 Fx App
- **THEN** Fx event logger MUST 由已注入的 `*zap.Logger` 构造，并输出到统一结构化日志链路
- **AND** 常规构图、执行前后、module trace 或 lifecycle 事件 MUST 使用 debug 级别，构造、Invoke、rollback 或 lifecycle 失败事件 MUST 使用 error 级别
- **AND** event logger MUST NOT 在 `LogEvent` 路径执行网络 I/O、远程导出、阻塞式重试或业务副作用
- **AND** Fx event logger MUST NOT 替换进程级默认 logger 或引入额外同步生命周期
