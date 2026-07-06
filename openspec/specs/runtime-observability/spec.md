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

系统 MUST 为共享 datastore 创建的 go-redis client 安装 OpenTelemetry tracing hook，使 Redis 命令在调用方 context 包含有效 trace 时产生 Redis client span。该 tracing MUST 不改变 Redis key schema、命令结果、连接生命周期、启动 ping、Redis PING metrics 或业务缓存语义。

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

