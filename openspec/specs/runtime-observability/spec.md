## Purpose

定义 user-service 和共享 runtime 的可观测性能力，覆盖健康检查、OpenAPI、metrics、tracing、日志、错误可观测性和部署观测资产。

## Requirements

### Requirement: 服务健康检查

系统 MUST 暴露 `/livez`、`/readyz` 和 `/startupz` 健康检查能力，用于报告 user-service runtime 及关键依赖状态。

#### Scenario: 服务健康

- **WHEN** HTTP 服务和关键依赖均可用
- **THEN** 健康检查 MUST 返回成功状态并包含服务名称或依赖检查信息

#### Scenario: 存活检查

- **WHEN** 调用 `/livez`
- **THEN** endpoint MUST 只证明进程可响应，并 MAY 在 PostgreSQL、Redis 或 RBAC policy 状态异常时继续返回成功

#### Scenario: 依赖异常

- **WHEN** PostgreSQL、Redis、Casbin policy 或 RBAC policy watcher 等被配置为就绪或启动检查的依赖不可用
- **THEN** `/readyz` 或 `/startupz` MUST 返回失败状态并暴露可定位的依赖错误信息，且 MUST NOT 暴露 secret、stacktrace、DSN、SQL、token 或 Cookie

#### Scenario: 路由注册

- **WHEN** user-service 启动并注册 HTTP 路由
- **THEN** 健康检查路由 MUST 在业务 API 外可访问，并不依赖业务授权中间件

### Requirement: OpenAPI 文档

系统 MUST 暴露和生成 OpenAPI 3 文档，覆盖认证会话、用户资料、角色管理、权限目录、RBAC 授权保护接口和健康检查。

#### Scenario: 访问 OpenAPI

- **WHEN** 调用方访问 OpenAPI 文档路由
- **THEN** 系统 MUST 返回与当前 user-service HTTP API 匹配的 OpenAPI 内容

#### Scenario: 生成 OpenAPI 文件

- **WHEN** 执行 `make user-service-openapi-generate`
- **THEN** 系统 MUST 更新 `user-service/docs/openapi.json`、`user-service/docs/openapi.yaml` 和相关生成文件

#### Scenario: OpenAPI drift

- **WHEN** API 注解或路由行为变化但 OpenAPI 生成物未同步
- **THEN** `make verify` MUST 能通过生成后 `git diff --exit-code` 暴露 drift

#### Scenario: 运行时文档路由归属

- **WHEN** user-service 暴露 OpenAPI UI、JSON 或 docs redirect
- **THEN** 路由 MUST 由 `user-service/internal/router/openapi.go` 拥有，且健康检查或 metrics endpoint MUST NOT 被当作 `/api/v1` 下的 feature 业务 API

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

### Requirement: Redis metrics 探测上下文传播

系统 MUST 在 metrics endpoint 处理 HTTP scrape 时，将本次 scrape request context 传播到 Redis runtime metrics 的 PING 探测，并继续使用配置化或默认 timeout 约束单次探测耗时。Redis metrics 的 metric family、label key、label value 和数值语义 MUST 保持稳定。

#### Scenario: scrape 取消终止 Redis PING

- **WHEN** metrics endpoint 正在执行 Redis PING 探测且 HTTP scrape request context 被取消
- **THEN** Redis PING MUST 观察到取消信号并尽快结束
- **AND** 系统 MUST NOT 因已取消 scrape 继续持有无意义的 Redis PING IO 直到外部网络超时

#### Scenario: Redis 探测 timeout 保留

- **WHEN** HTTP scrape request context 未取消但 Redis PING 超过 collector 配置的 timeout
- **THEN** Redis PING MUST 按 collector timeout 结束并记录 Redis 不可用快照
- **AND** `aegiscore_redis_up`、`aegiscore_redis_ping_duration_seconds` 和 `aegiscore_redis_ping_failures_total` 的名称、标签和含义 MUST 保持不变

#### Scenario: 最小探测间隔保留

- **WHEN** 连续 scrape 发生在 Redis collector 的最小探测间隔内
- **THEN** collector MUST 复用最近一次 Redis PING 快照
- **AND** 复用快照不得改变既有 metric family 或 label 契约

### Requirement: Metrics no-op 生成规范

系统 MUST 通过业务中立生成能力或生成规范为 feature-local metrics interface 生成 no-op 实现。业务 metrics interface MUST 保留在所属 feature 边界内，`common/runtime/observability/metrics` MUST NOT 承载 user-service 的 auth、permission、role、user 或其他服务业务指标方法。

#### Scenario: 生成 feature metrics no-op

- **WHEN** feature 定义业务 `Metrics` interface 且需要默认空实现
- **THEN** 系统 MUST 通过统一生成流程生成 no-op 实现文件
- **AND** feature MAY 保留 `NopMetrics()` 作为本 feature 的空实现入口
- **AND** 生成文件 MUST 与对应 feature-local `Metrics` interface 编译匹配

#### Scenario: common 保持业务中立

- **WHEN** `common/runtime/observability/metrics` 提供 metrics no-op 生成能力或规范
- **THEN** 该能力 MUST 只处理 Go interface 签名和空方法生成等业务中立逻辑
- **AND** 该能力 MUST NOT 定义 auth 登录、refresh、RBAC policy reload、route diff 或任何 user-service 业务指标方法

#### Scenario: 指标运行时语义不变

- **WHEN** 手写 metrics no-op 实现迁移为生成文件
- **THEN** Prometheus metric family、label key、label value、低基数约束和 tracing/logging 语义 MUST 保持不变
- **AND** metrics 配置禁用时的 no-op 行为 MUST 继续不产生运行时副作用

### Requirement: 本地缓存运行时指标

系统 MUST 为 `common/runtime/localcache` 提供 Prometheus metrics collector，导出低基数本地缓存运行时指标，用于观察命中率、回源率、回源错误、singleflight 合并、内部 double-check、写入丢弃、准入拒绝、淘汰和容量。部署观测资产、真实 metrics load 脚本和运行手册 MUST 消费当前稳定的 `aegiscore_localcache_*` metric family，并 MUST NOT 保留旧 metric name、旧 label 或兼容 PromQL。

#### Scenario: 导出本地缓存请求指标

- **WHEN** metrics 配置启用且服务注册 localcache collector
- **THEN** 系统 MUST 导出本地缓存 hit 和 miss counter
- **AND** 指标标签 MUST 只包含固定缓存名和固定枚举结果

#### Scenario: 导出本地缓存回源指标

- **WHEN** `GetOrLoad` 因 miss 执行 loader 或 loader 返回错误
- **THEN** 系统 MUST 导出 loader 调用和 loader 错误 counter
- **AND** 指标 MUST NOT 包含用户 ID、角色 ID、权限 ID、token、raw key、SQL、Redis key 或原始错误

#### Scenario: 导出防击穿指标

- **WHEN** `singleflight` 合并同 key 并发 miss 或内部 double-check 命中
- **THEN** 系统 MUST 导出 shared result 和 double-check hit counter
- **AND** 这些指标 MUST NOT 计入业务缓存 hit ratio

#### Scenario: 导出淘汰与拒绝指标

- **WHEN** Ristretto 丢弃写入、拒绝准入或淘汰缓存项
- **THEN** 系统 MUST 导出 set dropped、admission rejected 和 evicted counter
- **AND** SRE MUST 能通过这些指标判断容量、TTL 或 key 基数是否需要调整

#### Scenario: dashboard 消费 localcache 指标

- **WHEN** Grafana dashboard 展示 user-service runtime 依赖状态
- **THEN** dashboard MUST 使用 `aegiscore_localcache_requests_total`、`aegiscore_localcache_loads_total`、`aegiscore_localcache_singleflight_total`、`aegiscore_localcache_writes_total`、`aegiscore_localcache_evictions_total` 和 `aegiscore_localcache_capacity`
- **AND** PromQL MUST 按 `cache`、`result` 或 `event` 等固定 label 聚合，不得引用旧 metric name、旧 label 或 raw cache key

#### Scenario: alert 消费 localcache 指标

- **WHEN** Prometheus alert rules 评估本地缓存异常
- **THEN** alert MUST 使用当前 `aegiscore_localcache_*` metric family 表达 loader error、set dropped、admission rejected 或 eviction pressure 等可行动信号
- **AND** alert annotation MUST 指向稳定 runbook，并说明优先检查容量、TTL、key 基数、回源依赖和缓存注册状态

#### Scenario: metrics load 验证 localcache 指标

- **WHEN** 执行真实 metrics load 脚本采样 `/metrics` 和 Prometheus 查询
- **THEN** 脚本 MUST 检查 localcache metric family 的服务端 presence
- **AND** Prometheus sample query MUST 覆盖请求、回源、singleflight、写入、淘汰和容量指标，确保 collector 缺失或 PromQL 漂移能够被发现

#### Scenario: metrics 禁用

- **WHEN** metrics provider 被禁用或未配置
- **THEN** 系统 MUST 不注册 localcache Prometheus collector
- **AND** localcache 自身 MUST 继续维护可通过 `Stats()` 读取的本地统计快照

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

### Requirement: 日志与错误可观测性

系统 MUST 使用共享 logger 和 HTTP middleware 输出结构化日志，并在错误路径记录可关联的请求、span 和业务错误信息。

#### Scenario: 请求日志

- **WHEN** HTTP 请求被处理
- **THEN** 系统 MUST 记录方法、路径、状态码、耗时和关联上下文字段，日志字段名 MUST 使用稳定英文 `snake_case`，log message MUST 使用英文

#### Scenario: panic recovery

- **WHEN** HTTP handler 或 middleware 发生 panic
- **THEN** 系统 MUST 通过 recovery middleware 捕获 panic、记录错误并返回一致错误响应

#### Scenario: span error

- **WHEN** 业务错误需要关联 tracing span
- **THEN** 系统 MUST 使用共享 span error helper 记录错误状态

#### Scenario: 敏感日志

- **WHEN** 记录认证失败、请求错误或系统异常
- **THEN** 日志和 span event MUST NOT 记录 password、token、Authorization header、Cookie、原始请求体、DSN、SQL、Redis key 或敏感原始错误

#### Scenario: 日志等级

- **WHEN** 发生预期业务拒绝
- **THEN** 日志 MUST NOT 使用 `Error` 级别；当发生系统异常、外部依赖失败、后台任务失败或 panic recover 时，日志 MUST NOT 降级为 `Info`

### Requirement: 部署观测资产

系统 MUST 维护 Prometheus alerts、Grafana dashboards 和 Compose/Kubernetes/Helm 观测配置，使本地和集群环境能够查看 user-service 运行状态。

#### Scenario: 本地 Compose 观测

- **WHEN** 使用 `deployments/compose` 启动本地观测环境
- **THEN** Prometheus 和 Grafana MUST 能加载 user-service 相关 scrape、dashboard 和 datasource 配置

#### Scenario: dashboard 生成

- **WHEN** 执行 `make compose-dashboard-generate`
- **THEN** 系统 MUST 从通用观测 dashboard 生成 Compose Grafana dashboard

#### Scenario: dashboard drift 检查

- **WHEN** 通用 dashboard 和 Compose dashboard 不一致
- **THEN** `make compose-dashboard-check` MUST 能报告 drift
