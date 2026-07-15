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

系统 MUST 暴露和生成 OpenAPI 3 文档，覆盖认证会话、用户资料、角色管理、权限目录、RBAC 授权保护接口和健康检查。OpenAPI 文档 MUST 与 user-service 当前 HTTP API 的响应 shape 保持一致，尤其是登录接口 MUST 同步表达普通登录和强制改密登录两种 envelope 语义。运行时 OpenAPI UI MUST 使用 `github.com/swaggo/files/v2` 的 embedded `fs.FS` 提供 Swagger UI 静态资源，MUST NOT 保留 `github.com/swaggo/files` v1 import、旧 handler fallback 或 v1/v2 双写兼容分支。

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

#### Scenario: Swagger UI 使用 v2 静态资源

- **WHEN** user-service 注册 OpenAPI UI 路由
- **THEN** `/openapi/*any` MUST 使用 `github.com/swaggo/files/v2` 的 embedded `fs.FS` 提供 Swagger UI 静态资源
- **AND** 生产代码 MUST NOT import `github.com/swaggo/files` v1 模块路径
- **AND** 生产代码 MUST NOT 保留 v1 handler fallback、`gin-swagger` wrapper、版本探测分支或旧静态资源兼容路径

#### Scenario: 登录接口文档表达 envelope 分支

- **WHEN** user-service 生成 OpenAPI 文档
- **THEN** 登录接口 MUST 声明普通登录响应携带 `success=true`、`CodeOK`、access token、refresh token、token type 和 expires_in
- **AND** 登录接口 MUST 描述强制改密登录响应携带 `success=false`、`CodePasswordChangeRequired`、受限 access token、token type 和 expires_in
- **AND** 登录接口 MUST 复用 `TokenResponse` schema，MUST NOT 声明单独的 `LoginResponse` schema
- **AND** 登录接口 MUST NOT 声明 `status`、`authenticated` 或 `password_change_required` 响应枚举
- **AND** 登录接口 MUST 继续声明 KDF busy 可能返回 `503 Service Unavailable`

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

### Requirement: Redis metrics 探测上下文传播

系统 MUST 在 metrics endpoint 通过 `metrics.Provider.HTTPHandler` 或 `metrics.Provider.GatherContext` 处理 HTTP scrape 时，将本次 scrape request context 传播到 Redis runtime metrics 的 PING 探测，并继续使用配置化或默认 timeout 约束单次探测耗时。Redis metrics collector 的标准 Prometheus `Collect` 实现 MUST 作为 background context fallback，MUST NOT 声明可感知 HTTP scrape 取消。Redis metrics 的 metric family、label key、label value 和数值语义 MUST 保持稳定。

#### Scenario: provider scrape 取消终止 Redis PING

- **WHEN** metrics endpoint 经 `metrics.Provider.HTTPHandler` 或 `metrics.Provider.GatherContext` 执行 Redis PING 探测且 HTTP scrape request context 被取消
- **THEN** Redis PING MUST 观察到取消信号并尽快结束
- **AND** 系统 MUST NOT 因已取消 scrape 继续持有无意义的 Redis PING IO 直到外部网络超时

#### Scenario: 标准 Collect fallback 不声明 scrape 取消

- **WHEN** Redis metrics collector 被直接通过 Prometheus 标准 `Collect` 调用，且没有经过 `metrics.Provider.GatherContext`
- **THEN** collector MUST 使用 background context 与 collector timeout 执行探测
- **AND** collector MUST NOT 声明或依赖 HTTP scrape request context 取消语义

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

系统 MUST 为 HTTP 请求提供可由调用方观察的 request ID 关联能力。入站请求携带合法 `X-Request-ID` 时系统 MUST 透传该值；缺失或不合法时系统 MUST 生成新的 request ID。最终 request ID MUST 写入响应头 `X-Request-ID`，并 MUST 由通用 logger context 以 `request_id` 字段关联 HTTP access log 和请求生命周期内通过共享 logger 记录的应用日志。

#### Scenario: 透传入站请求 ID

- **WHEN** HTTP 请求携带合法 `X-Request-ID`
- **THEN** 系统 MUST 在响应头 `X-Request-ID` 中回传相同值
- **AND** HTTP access log 和请求生命周期内的共享 logger 日志 MUST 使用 `request_id` 字段记录相同值

#### Scenario: 生成缺失请求 ID

- **WHEN** HTTP 请求未携带 `X-Request-ID`
- **THEN** 系统 MUST 生成新的请求 ID
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

### Requirement: common HTTP 可观测性测试断言规范

系统 MUST 在 `common/http` 的 OpenAPI、pprof 和 middleware 可观测性相关测试中使用语义化 `testify` 断言验证当前稳定输出。测试 MUST 聚焦当前 OpenAPI 输出、pprof route 和 middleware 可观测性行为，不得新增旧 header、旧 CORS、旧 OpenAPI 输出或旧 pprof route 兼容断言。

#### Scenario: 验证 OpenAPI 输出

- **WHEN** `common/http/openapi` 测试验证 OpenAPI JSON、YAML、HTML 或转换结果
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.JSONEq`、`require.Equal`、`require.Contains` 或结构化解析后的语义化断言
- **AND** 测试 MUST NOT 通过 `Fail*`、`t.Fatal*` 或 `t.Error*` 表达可语义化验证的输出差异

#### Scenario: 验证 pprof route

- **WHEN** `common/http/pprof` 测试验证 pprof route 注册、HTTP status 或响应 header
- **THEN** 测试 MUST 使用 `require` 或必要的 `assert` 语义化断言表达当前 route 行为
- **AND** 测试 MUST NOT 新增旧 pprof route 兼容断言或测试专用生产分支

#### Scenario: 验证 middleware 可观测性输出

- **WHEN** `common/http/middleware` 测试验证日志、metrics、request ID、recovery、CORS 或 tracing 相关 HTTP 输出
- **THEN** 前置条件和依赖性检查 MUST 使用 `require`
- **AND** 只有多个相互独立的响应字段需要一次性收集失败时 MAY 使用 `assert`
- **AND** 测试 MUST 验证当前稳定行为，不得双写旧 header、旧 CORS 或旧 envelope 断言

#### Scenario: 验证 common HTTP 测试通过

- **WHEN** OpenAPI、pprof 和 middleware 断言迁移完成
- **THEN** `go test ./common/http/...` MUST 通过
- **AND** `openspec validate standardize-common-http-test-assertions-no-compat` MUST 通过

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

### Requirement: CORS middleware 回归测试不改变观测链路

系统 MUST 在补齐 `common/http/middleware` 中 CORS 默认入口测试时保持既有 runtime observability middleware 行为不变。测试补齐 MUST 限定在当前 CORS 默认策略与自定义策略稳定字段，不得修改 request ID、logging、metrics、tracing、recovery、pprof、OpenAPI 路由或 user-service 运行时 middleware 挂载策略。

#### Scenario: 观测 middleware 行为保持不变

- **WHEN** 为 `common/http/middleware.CORS()` 或 `CORSWithOptions` 增加测试覆盖
- **THEN** 系统 MUST NOT 修改 request ID、logging、metrics、tracing、recovery、pprof 或 OpenAPI 相关生产代码
- **AND** 系统 MUST NOT 改变这些 middleware 的 HTTP status、header、日志字段、metrics label 或 tracing span 语义

#### Scenario: 服务挂载策略保持不变

- **WHEN** CORS 默认入口测试补齐完成
- **THEN** user-service HTTP router 的 CORS 挂载策略 MUST 保持不变
- **AND** 本次 change MUST NOT 新增、删除或移动 user-service 运行时 CORS middleware 挂载点

### Requirement: user-service runtime observability 测试断言迁移

`user-service/internal/router` 与 `user-service/internal/providers` 中覆盖 health、metrics、OpenAPI、pprof、Gin middleware、日志、tracing 和 runtime endpoint 的测试 MUST 使用统一断言规范验证运行时观测行为。断言迁移 MUST 保持健康探针路径、metrics endpoint、OpenAPI 文档路由、pprof 路由、Prometheus metric family、label key/value、日志字段、tracing span 和低噪声 runtime endpoint 过滤语义不变。

#### Scenario: health 和 runtime route 断言

- **WHEN** router 或 provider 测试验证 `/livez`、`/readyz`、`/startupz`、metrics endpoint、OpenAPI UI/JSON、docs redirect 或 pprof 路由
- **THEN** 测试 MUST 使用 `require` 或必要时 `assert` 表达 HTTP status、响应 JSON、路径注册、Content-Type、Location、service name、checks 顺序和低噪声路由判断
- **AND** 迁移 MUST NOT 新增旧 metrics path、旧 pprof path、旧 OpenAPI route alias 或旧 health route 兼容断言

#### Scenario: metrics、日志和 tracing 结构化断言

- **WHEN** provider 或 Gin middleware 测试验证 Prometheus metric family、label、sample 值、请求日志字段、panic recovery 日志、span status、span event 或 trace/request ID 传播
- **THEN** 测试 MUST 优先使用 `require.Len`、`require.Equal`、`require.Contains`、`require.NotContains`、`require.Greater`、`require.Regexp` 或等价语义化断言
- **AND** 多个互相独立的 metric、label 或日志字段检查 MAY 使用 `assert`
- **AND** 迁移 MUST NOT 改变指标名称、label key/value、日志 message、稳定英文 `snake_case` 字段名或 tracing 上下文传播语义

#### Scenario: context 和取消路径断言

- **WHEN** metrics scrape、health check 或 Gin middleware 测试验证 request context、timeout、取消和 runtime endpoint skip 行为
- **THEN** 测试 MUST 使用语义化断言表达 context error、耗时边界、状态码和副作用计数
- **AND** 对 goroutine handoff、channel 协调或取消竞态的特殊控制流例外 MUST 在 change tasks 中列明

### Requirement: runtime observability 测试不得改变生产观测契约

断言迁移 MUST 只改变 `_test.go` 中的失败表达方式。系统 MUST NOT 为了满足测试断言迁移而修改 runtime observability 生产代码、生成物或部署资产。

#### Scenario: 生产观测行为保持不变

- **WHEN** health、metrics、OpenAPI、pprof、Gin middleware、logger 或 tracing 相关测试迁移历史断言
- **THEN** 生产路由注册、handler、middleware、metrics collector、OpenAPI 生成物、日志字段和 tracing provider 行为 MUST 保持不变
- **AND** change MUST NOT 修改 `user-service/docs/openapi.*`、Prometheus/Grafana 部署资产或 runtime metrics 输出格式

### Requirement: 用户服务聚合运行时路由注册测试
系统 MUST 使用 router 包测试覆盖 user-service 聚合运行时路由注册结果，确保健康检查、OpenAPI、metrics 和 pprof 路由保持在当前路径和当前授权边界内。

#### Scenario: 健康检查与 OpenAPI 路由注册
- **WHEN** `RegisterUserServiceHTTPRoutes` 使用当前配置注册 HTTP 路由
- **THEN** 测试 MUST 验证 `/livez`、`/readyz`、`/startupz`、OpenAPI JSON 和 OpenAPI UI 或 redirect 路由注册在 `/api/v1` 外
- **AND** 测试 MUST 验证这些运行时路由不依赖业务认证或 RBAC 授权中间件

#### Scenario: metrics 配置错误返回
- **WHEN** metrics endpoint 配置为与健康检查、OpenAPI、`/api/v1` 或 pprof 保留前缀冲突的路径
- **THEN** `RegisterUserServiceHTTPRoutes` MUST 返回 metrics 配置错误
- **AND** 测试 MUST NOT 接受旧 metrics path 或旧兼容别名作为成功路径

#### Scenario: pprof 开关影响注册结果
- **WHEN** pprof 配置禁用
- **THEN** 测试 MUST 验证当前 pprof base path 未注册
- **AND** 当 pprof 配置启用时，测试 MUST 验证仅当前配置的 pprof base path 和 profile wildcard 被注册

### Requirement: E2E runtime harness 断言规范
系统 MUST 保持 user-service E2E harness 对运行时启动、配置、日志目录、Gin engine、HTTP request 构造和 response envelope 解码的现有语义，并 MUST 使用 `docs/TESTING.md` 规定的语义化断言表达 harness 前置条件和失败诊断。

#### Scenario: E2E 环境开关保持不变
- **WHEN** E2E harness 判断是否运行 HTTP flow 集成测试
- **THEN** 测试 MUST 保持当前 `AEGISCORE_TEST_E2E` 和通用 Testcontainers 开关语义
- **AND** 未启用容器测试时 MUST 继续跳过，而不是通过新断言改变运行前置条件

#### Scenario: runtime 启动和配置断言
- **WHEN** E2E harness 写入测试配置、分配本地端口、启动 PostgreSQL/Redis 容器、应用 migration、构造 Fx app 并填充 Gin engine
- **THEN** 测试 MUST 使用 `require.NoError`、`require.NotNil`、`require.NotEmpty`、`require.Greater` 或等价语义化断言表达前置条件
- **AND** 迁移 MUST NOT 改变日志配置、HTTP timeout、Redis/PostgreSQL 配置、Fx app 启停或 `bootstrap.AppModule` 装配语义

#### Scenario: response envelope 解码断言
- **WHEN** E2E harness 解码 HTTP response envelope、校验 status、`success`、应用错误码、message 和 `data`
- **THEN** 测试 MUST 使用语义化 `require` / 必要 `assert` 表达 JSON decode、字段值和空数据检查
- **AND** 迁移 MUST NOT 改变共享 response envelope、公开 message、HTTP status 或 JSON 字段名称

### Requirement: OpenAPI 转换工具测试断言
OpenAPI 转换和生成链路相关工具测试 MUST 使用语义化断言验证转换错误、OpenAPI JSON/YAML 内容、生成文件路径和生成物存在性。测试断言迁移 MUST NOT 改变 OpenAPI 文档路由、OpenAPI 生成物、Swagger/OpenAPI 转换输出契约或服务专属生成参数。

#### Scenario: 验证 OpenAPI 转换输出
- **WHEN** 工具测试验证 Swagger 2 到 OpenAPI 3 的转换结果、JSON/YAML 输出或生成文件内容
- **THEN** 测试 MUST 优先使用 `require.JSONEq`、`require.Contains`、`require.ElementsMatch`、`require.Len`、`require.Regexp` 或等价语义化断言
- **AND** 测试 MUST NOT 使用手写字符串拼接失败消息或布尔包装替代已有专属断言

#### Scenario: 保持 OpenAPI 生成契约不变
- **WHEN** 迁移 OpenAPI 转换工具测试断言
- **THEN** 系统 MUST NOT 修改 `make user-service-openapi-generate` 的输出文件集合
- **AND** 系统 MUST NOT 修改 OpenAPI UI/JSON 路由、认证方案、扫描范围、CLI flag 或服务脚本传入的生成参数

### Requirement: Redis 命令 tracing

系统 MUST 为共享 datastore 创建的 go-redis client 安装 OpenTelemetry tracing hook，使 Redis 命令在调用方 context 包含有效 trace 时产生 Redis client span。user-service Redis provider MUST 使用服务级 tracing provider 注入 go-redis instrumentation，缺少 tracing provider 时 MUST 返回明确错误。该 tracing MUST 不改变 Redis key schema、命令结果、连接生命周期、启动 ping、Redis PING metrics 或业务缓存语义。

#### Scenario: Redis 命令产生 span

- **WHEN** 服务在有效 OpenTelemetry trace context 中通过共享 datastore Redis client 执行 Redis 命令
- **THEN** 系统 MUST 产生 Redis client span 并关联到当前 trace
- **AND** span MUST 使用 OpenTelemetry Redis instrumentation 的低基数属性

#### Scenario: Redis tracing 禁止敏感字段

- **WHEN** 系统记录 Redis command span 或 span event
- **THEN** span 属性 MUST NOT 暴露 Redis key、token、用户 ID、角色 ID、权限 ID、原始错误、密码或连接 DSN
- **AND** metrics 标签 MUST 继续遵守低基数约束

#### Scenario: Redis tracing 不改变禁用行为

- **WHEN** tracing provider 未启用或使用 no-op tracer provider
- **THEN** Redis 命令 MUST 保持原有执行结果和错误语义
- **AND** 系统 MUST NOT 因 tracing 禁用跳过 Redis 命令、启动 ping 或 Redis metrics 探测

#### Scenario: Redis provider 注入服务 tracing provider

- **WHEN** user-service 通过 Redis provider 创建共享 datastore Redis client
- **THEN** provider MUST 将服务级 tracing provider 显式传递给 Redis instrumentation
- **AND** 缺少 tracing provider 时 provider MUST 返回明确错误并拒绝继续装配 Redis client
- **AND** Redis 启动 PING 产生 tracing span 时 MUST 保持 PING 命令结果、连接生命周期和 metrics 语义不变

### Requirement: Ent 查询观测

系统 MUST 为 user-service Ent query 导出 OpenTelemetry query span、query latency histogram 和 query error counter。Ent query 观测 MUST 位于服务级 Ent client/provider 边界或服务级观测代码中，不得手写修改 `user-service/ent/` 生成代码，不得把 user-service Ent entity 语义放入 `common/runtime/datastore`。

#### Scenario: Ent 查询产生 span

- **WHEN** 服务在有效 OpenTelemetry trace context 中执行 Ent query
- **THEN** 系统 MUST 产生 Ent query span 并关联到当前 trace
- **AND** span 属性 MUST 使用低基数 query/entity 信息
- **AND** span MUST NOT 记录 raw SQL、SQL 参数、用户 ID、角色 ID、权限 ID、token、DSN 或原始错误文本

#### Scenario: Ent 查询 latency 指标

- **WHEN** Ent query 执行完成
- **THEN** 系统 MUST 将本次 query 耗时写入 Ent query latency histogram
- **AND** histogram 标签 MUST 使用稳定低基数 entity/query/result 枚举
- **AND** histogram 标签 MUST NOT 包含 raw SQL、SQL 参数、用户 ID、角色 ID、权限 ID、trace/span ID 或原始错误

#### Scenario: Ent 查询错误指标

- **WHEN** Ent query 返回错误
- **THEN** 系统 MUST 增加 Ent query error counter
- **AND** error counter 标签 MUST 使用稳定低基数 entity/query 枚举
- **AND** 系统 MUST 保持原始 query error 返回语义不变

#### Scenario: Ent 观测不修改数据库契约

- **WHEN** 系统新增 Ent query tracing 和 metrics
- **THEN** 系统 MUST NOT 修改 Ent schema、Atlas migration、SQL 表结构、索引、OpenAPI 或 HTTP API
- **AND** SQL 连接池 metrics 的 metric family、label key、label value 和数值语义 MUST 保持不变

### Requirement: RBAC Enforce 延迟 dashboard

系统 MUST 在 user-service Grafana dashboard 中展示 RBAC Enforce latency histogram 的分位延迟，使 SRE 和开发者能够按授权结果、HTTP 方法和路由模板观察 RBAC 授权判定慢尾延迟。dashboard MUST 直接消费当前稳定的 `aegiscore_user_service_rbac_enforce_duration_seconds` metric family，并 MUST NOT 保留旧指标名、旧 label 或兼容 PromQL。

#### Scenario: 展示 RBAC Enforce P95 和 P99 延迟

- **WHEN** Grafana 加载 user-service overview dashboard
- **THEN** dashboard MUST 包含 RBAC Enforce P95/P99 延迟面板
- **AND** 面板 MUST 使用 `aegiscore_user_service_rbac_enforce_duration_seconds_bucket` 计算 `histogram_quantile(0.95, ...)` 和 `histogram_quantile(0.99, ...)`

#### Scenario: RBAC Enforce 延迟查询保持低基数

- **WHEN** dashboard 查询 RBAC Enforce latency histogram
- **THEN** PromQL MUST 只按 `le`、`method`、`route_template`、`result` 以及固定 `service`、`environment` 过滤条件聚合
- **AND** PromQL MUST NOT 引用用户 ID、角色 ID、权限 ID、会话 ID、token ID、trace/span ID、raw path、IP、SQL、Redis key 或原始错误

#### Scenario: Compose dashboard 同步 RBAC Enforce 延迟面板

- **WHEN** 执行 `make compose-dashboard-generate`
- **THEN** Compose Grafana dashboard MUST 包含与通用 dashboard 相同的 RBAC Enforce P95/P99 延迟面板和 PromQL
- **AND** 除 Prometheus datasource uid 外，Compose dashboard MUST 与通用 dashboard 保持结构一致

### Requirement: 强制改密安全指标与告警

系统 MUST 为强制改密一次性会话和安全撤销链路提供 Prometheus 指标与告警。指标标签 MUST 保持低基数，MUST NOT 包含用户 ID、session ID、jti、token、Redis key、SQL、IP、用户名、邮箱、trace/span ID、原始错误或 stacktrace。

#### Scenario: 一次性会话消费失败指标
- **WHEN** password-change session 消费因不存在、过期、撤销、复用或 claims 不一致而失败
- **THEN** 系统 MUST 递增强制改密会话消费失败指标
- **AND** 指标标签 MUST 只使用固定枚举原因
- **AND** 指标 MUST NOT 暴露具体用户、session 或 token 标识

#### Scenario: 重复消费拒绝指标
- **WHEN** 同一个 password-change token 被重复使用并被拒绝
- **THEN** 系统 MUST 记录可聚合的重复消费拒绝指标
- **AND** SRE MUST 能基于该指标发现 token 重放或客户端重试异常

#### Scenario: 撤销投影失败指标
- **WHEN** 强制改密成功更新凭据后，本地 token version cache 失效、Redis token version 投影刷新或 refresh session 删除任一步骤失败
- **THEN** 系统 MUST 递增强制改密撤销投影失败指标
- **AND** 指标 MUST 能区分失败步骤的固定枚举类型
- **AND** 指标 MUST NOT 包含原始错误文本或高基数标识

#### Scenario: 补偿失败指标
- **WHEN** 系统尝试记录或执行强制改密撤销补偿但失败
- **THEN** 系统 MUST 递增强制改密撤销补偿失败指标
- **AND** 系统 MUST 记录包含固定错误分类的日志
- **AND** 日志 MUST NOT 包含 token、jti、session ID 或 Redis key 明文

#### Scenario: 告警覆盖安全撤销失败
- **WHEN** 强制改密撤销投影失败或补偿失败指标在观察窗口内大于 0
- **THEN** Prometheus alert rules MUST 产生可行动告警
- **AND** 告警说明 MUST 指向稳定 runbook 或排查说明
- **AND** 告警 MUST 提示优先检查 Redis、token version 投影、本地缓存失效和 refresh session 删除链路

#### Scenario: metrics load 验证强制改密指标
- **WHEN** 执行观测资产或 metrics load 验证脚本
- **THEN** 验证 MUST 覆盖强制改密会话消费失败、重复消费拒绝、撤销投影失败和补偿失败指标的 presence 或 PromQL 查询
- **AND** 指标缺失或 PromQL drift MUST 能被验证流程发现

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

### Requirement: HTTP 服务与观测端点

系统 MUST 使用 `server.http` 驱动 HTTP server 生命周期，metrics 路径校验 MUST 与 pprof 配置解耦，pprof MUST 默认关闭且不得通过业务 HTTP router 暴露。

#### Scenario: HTTP server 禁用

- **WHEN** `server.http.enabled=false`
- **THEN** 服务 MUST NOT 启动 HTTP listener
- **AND** 禁用状态 MUST NOT 要求 HTTP host、port 或 timeout 使用非零占位值

#### Scenario: 显式启用 pprof

- **WHEN** 运维人员通过显式诊断入口或 `PPROF_ENABLED` 启用 pprof
- **THEN** pprof MUST 使用独立 listener
- **AND** `PPROF_ADDR` 未配置时 MUST 使用 `127.0.0.1:6060`
- **AND** production 环境 MUST 拒绝非 loopback 监听地址

### Requirement: 云原生日志输出

系统 MUST 将应用日志输出到 stdout/stderr，核心 `LogConfig` MUST 只包含 `level` 和 `format`，MUST NOT 实现应用内文件拆分或轮转。

#### Scenario: 结构化日志分类

- **WHEN** 应用、HTTP、SQL 或 Redis 组件记录日志
- **THEN** 日志 MUST 使用稳定的 `logger` 和 `component` 字段区分来源
- **AND** 关联上下文 SHOULD 包含 service、env、trace_id、span_id、request_id 和 error

#### Scenario: Ent SQL 日志分级

- **WHEN** Ent 执行普通、慢查询或失败 SQL
- **THEN** 普通 SQL MUST 为 debug，慢 SQL MUST 为 warn，失败 SQL MUST 为 error
- **AND** 日志 MUST 包含 logger、component、db、operation、duration_ms 和 error

### Requirement: OTLP tracing 最小配置

系统 MUST 仅通过 enabled、sample_ratio、otlp_endpoint 和 insecure 配置 tracing，MUST NOT 暴露 exporter 选择字段。

#### Scenario: tracing 关闭

- **WHEN** tracing disabled
- **THEN** 服务 MUST NOT 初始化 OTLP exporter

#### Scenario: tracing 启用

- **WHEN** tracing enabled
- **THEN** otlp_endpoint MUST 非空
- **AND** insecure MUST 只表达 OTLP transport 是否使用明文

### Requirement: 监听服务内部故障触发失败关闭信号

user-service 的 HTTP 与 pprof listener/server 在 Fx App 成功启动后发生非预期退出时，系统 MUST 记录可诊断错误并调用 `fx.Shutdowner.Shutdown(fx.ExitCode(1))`。正常生命周期关闭产生的预期 Serve 结果 MUST NOT 触发失败 shutdown signal。

#### Scenario: HTTP Serve 非预期退出

- **WHEN** HTTP server 的 `Serve` 在生命周期未进入正常停止状态时返回非预期 listener 或服务错误
- **THEN** bootstrap MUST 调用 `Shutdown(fx.ExitCode(1))`
- **AND** 错误日志 MUST 保留 HTTP server 故障原因

#### Scenario: pprof Serve 非预期退出

- **WHEN** 已启用的独立 pprof server 因非预期 listener 关闭或服务错误退出
- **THEN** bootstrap MUST 调用 `Shutdown(fx.ExitCode(1))`
- **AND** 错误日志 MUST 保留 pprof server 故障原因

#### Scenario: 正常关闭监听服务

- **WHEN** Fx 生命周期停止 HTTP 或 pprof server，且 `Serve` 返回 `http.ErrServerClosed` 或能由生命周期取消证明的预期关闭错误
- **THEN** bootstrap MUST NOT 把该结果报告为内部故障
- **AND** bootstrap MUST NOT 因该结果额外调用失败 shutdown signal

#### Scenario: 请求关闭失败

- **WHEN** listener/server 故障发生后 `Shutdown(fx.ExitCode(1))` 返回错误
- **THEN** bootstrap MUST 记录 shutdown 请求失败及其错误原因
- **AND** bootstrap goroutine MUST NOT 直接调用 `App.Stop()` 或 `os.Exit`

### Requirement: user-service 优雅关闭总预算边界

user-service MUST 将 `runtime.lifecycle.stop_timeout` 视为 `app.Stop()` 和全部 Fx `OnStop` hook 的进程级总预算。Fx MUST 保持逆注册顺序串行停止组件；每个 hook MUST 使用同一 Stop context 或其派生的更短 context，单个 HTTP、workerpool、exporter 或 datastore 关闭 timeout MUST NOT 被解释为全部 hook 的总预算，也不得通过并行执行 hook 绕开资源关闭顺序。

默认关闭链路 MUST 在 120 秒 Fx 总预算内依次为 HTTP 请求排空、auth session purge workerpool、RBAC policy watcher、pprof、tracing、Ent/PostgreSQL/Redis 和 logger 同步提供关闭机会；具有 25 秒 HTTP 子预算和 30 秒 workerpool 子预算的 hook MUST 继续受 Fx 剩余 deadline 约束，没有独立子预算的 hook MUST 继续受同一 Fx Stop context 约束。

#### Scenario: 外部终止信号进入总预算关闭链路

- **WHEN** user-service 收到 `SIGINT` 或 `SIGTERM`
- **THEN** 进程 MUST 使用默认 120 秒 Stop context 调用一次 `app.Stop()`
- **AND** Fx MUST 在该 context 内按逆注册顺序串行执行已注册的 `OnStop` hook

#### Scenario: 内部故障进入同一关闭链路

- **WHEN** HTTP 或 pprof server 的非预期退出产生 Fx shutdown signal
- **THEN** 进程 MUST 使用与外部终止相同的 Fx Stop 总预算和组件关闭语义
- **AND** 部署 grace MUST 为 tracing flush、datastore 关闭和 logger sync 等后序工作保留到总预算结束后的平台余量

#### Scenario: 局部组件 timeout 不替代总预算

- **WHEN** HTTP shutdown timeout 为 25 秒、auth session purge workerpool StopTimeout 为 30 秒或 OTLP exporter I/O timeout 为 5 秒
- **THEN** 这些值 MUST 仅限制各自组件或 I/O 操作
- **AND** 运维和自动校验 MUST NOT 使用任一局部 timeout 作为 Kubernetes 或 Helm termination grace 的应用预算来源

#### Scenario: 前序 hook 消耗关闭时间

- **WHEN** 一个前序 `OnStop` hook 在 Fx 逆序串行关闭中消耗部分 Stop 时间
- **THEN** 后序 hook MUST 观察同一全局 deadline 的剩余时间
- **AND** 系统 MUST NOT 为每个 hook 重新创建完整 120 秒父预算

#### Scenario: 正常快速关闭

- **WHEN** HTTP 已无活跃请求、workerpool 已无任务、watcher 已退出且外部资源可立即关闭
- **THEN** 各 `OnStop` hook MUST 在完成自身关闭后立即返回
- **AND** 进程 MUST NOT 为耗尽 Fx Stop budget 或 Kubernetes termination grace 而主动等待

### Requirement: Permission Metrics 正式依赖图必须完整

user-service 的正式 permission 模块 MUST 向 `PermissionQueryService` 提供唯一且明确的单值 `permissionapplication.Metrics` 依赖。该依赖 MUST 在 Fx/Dig 图中作为必选输入边存在，MUST NOT 使用 variadic、optional 或 slice/group annotation 表达可缺失的 Metrics。

#### Scenario: metrics 启用时注入真实实现

- **WHEN** user-service 以 metrics 启用配置构造正式 App
- **THEN** permission 模块 MUST 向 `PermissionQueryService` 注入当前 Prometheus Metrics 实现
- **AND** route diff 查询 MUST 能更新既有 `aegiscore_user_service_permission_route_diff` 指标

#### Scenario: metrics 禁用时注入 Nop 实现

- **WHEN** user-service 以 metrics 禁用配置构造正式 App
- **THEN** permission 模块 MUST 向 `PermissionQueryService` 注入现有 `permissionapplication.NopMetrics()` 实现
- **AND** 正式 App MUST 完成构图且 MUST NOT 注册或更新 permission Prometheus 指标

#### Scenario: DOT 图展示明确 Metrics 输入边

- **WHEN** 测试生成包含正式 `permission.Module` 的 Fx/DOT 依赖图
- **THEN** `PermissionQueryService` 构造节点 MUST 存在明确的 `permissionapplication.Metrics` 输入边
- **AND** 依赖图 MUST NOT 依赖 variadic、错误的 optional 或 slice/group annotation 补偿该输入

#### Scenario: 指标契约保持不变

- **WHEN** permission Metrics 的正式依赖接线被修复
- **THEN** 既有 metric family、指标名称、label key、label value 和低基数约束 MUST 保持不变
- **AND** 系统 MUST NOT 新增 metrics backend、dashboard 或 alert

### Requirement: user-service Fx lifecycle timeout 同源与作用边界

user-service composition root MUST 使用同一份已解析 service config 的 `runtime.lifecycle.start_timeout` 和 `runtime.lifecycle.stop_timeout` 设置 App 顶层 `fx.StartTimeout` 与 `fx.StopTimeout`。`serve` 命令手动调用 `App.Start` 和 `App.Stop` 时 MUST 使用同一配置值创建显式 context；这些 context MUST 作为当前 CLI lifecycle hook 的实际 deadline，Fx App 顶层 timeout 与显式 context MUST NOT 被解释为可累加的两段预算。

`fx.StartTimeout` MUST NOT 被描述或实现为配置加载或 `fx.New` 同步构造阶段的 deadline。配置加载 MUST 在 `fx.New` 之前完成；对构造期 provider、invoke 或资源 I/O 的 timeout 与 lifecycle 迁移 MUST 由其自身 context 或后续独立 change 定义。

#### Scenario: App 与 CLI 使用相同 lifecycle 配置

- **WHEN** CLI 使用已解析 service config 创建正式 Fx App
- **THEN** App 的 Start/Stop timeout MUST 分别等于该配置的 `runtime.lifecycle.start_timeout` 和 `runtime.lifecycle.stop_timeout`
- **AND** CLI 传给 `App.Start` 与 `App.Stop` 的 context MUST 分别使用相同的两个配置值

#### Scenario: 显式 Start context 是实际启动边界

- **WHEN** `serve` 命令手动调用 `App.Start(startCtx)`
- **THEN** lifecycle `OnStart` hook MUST 接收受 `startCtx` 限制的启动预算
- **AND** App 顶层 `fx.StartTimeout` MUST NOT 在该 context 之外增加或串联第二段启动预算

#### Scenario: 显式 Stop context 是实际停止边界

- **WHEN** 外部 context 或内部 Fx shutdown signal 触发 `serve` 停止
- **THEN** CLI MUST 使用未被取消的上游 context value 和配置化 `stop_timeout` 调用一次 `App.Stop(stopCtx)`
- **AND** App 顶层 `fx.StopTimeout` MUST NOT 在该 context 之外增加或串联第二段停止预算

#### Scenario: fx.New 不受 StartTimeout 限制

- **WHEN** user-service 在 `fx.New` 中同步构建依赖图、执行 invoke 或解析其 constructor 依赖
- **THEN** 系统 MUST NOT 声称 `fx.StartTimeout` 会中断或限制该阶段
- **AND** 文档与配置注释 MUST 将 `start_timeout` 描述为配置加载后 `App.Start` lifecycle 阶段的预算

#### Scenario: 不隐式迁移构造期资源

- **WHEN** 本 change 设置 App 顶层 lifecycle timeout 并统一配置来源
- **THEN** 系统 MUST NOT 因此声称全部 provider constructor 或 invoke 已具备可取消的构造期 deadline
- **AND** 任何把构造期资源工作迁移到 `OnStart` 的行为 MUST 另行评估依赖顺序、回滚和测试

### Requirement: 正式 App logger 生命周期与显式来源

user-service 正式 App MUST 使用 Fx 装配出的服务级 `*zap.Logger` 作为运行时日志依赖来源，并由 logger provider 在 App Stop 阶段同步该正式 logger。正式 App MUST NOT 通过安装、恢复或持有进程级默认 logger 来表达 logger lifecycle；request lifecycle 中的日志关联 MUST 继续通过明确写入 request context 的 logger 和 request ID context 传播。

#### Scenario: App Stop 同步正式 logger
- **WHEN** user-service Fx App 停止并执行 logger provider 的 `OnStop` hook
- **THEN** 系统 MUST 对服务级正式 logger 执行既有 `Sync` 责任
- **AND** stdout/stderr 不支持 fsync 的平台错误 MUST 继续按既有规则忽略

#### Scenario: 正式 App 不安装默认 logger
- **WHEN** user-service 正式 App 构造或启动 logger provider
- **THEN** provider MUST NOT 调用 `logger.SetDefault` 或等价逻辑安装进程级默认 logger
- **AND** App Stop MUST NOT 恢复旧默认 logger 或持有默认 logger restore 状态

#### Scenario: 并行 App logger 隔离
- **WHEN** 同一进程并行或连续构造多个 user-service 测试 App
- **THEN** 每个 App 的 feature、middleware 和 provider 日志 MUST 来源于自身注入的服务级 logger 或 request context logger
- **AND** 一个 App 构造的 logger MUST NOT 覆盖另一个 App 通过默认 logger fallback 观察到的实例

#### Scenario: request context 日志关联保持不变
- **WHEN** HTTP 请求经过 request ID 和 tracing 相关 middleware 并进入业务处理
- **THEN** 请求生命周期内通过明确 context logger 记录的日志 MUST 继续包含有效的 `request_id`、`trace_id` 和 `span_id`
- **AND** 本变更 MUST NOT 修改 `X-Request-ID`、W3C `traceparent` 或 `tracestate` 的外部传播契约

#### Scenario: 观测输出契约保持不变
- **WHEN** logger 依赖来源从进程默认迁移为显式注入或 request context
- **THEN** 日志 message、level、logger name、`component`、`service`、`env`、`request_id`、`trace_id`、`span_id` 和敏感信息过滤语义 MUST 保持不变
- **AND** 系统 MUST NOT 修改 metrics、tracing、OpenAPI、pprof、Prometheus alert 或 Grafana dashboard 契约
