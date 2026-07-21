## Purpose

定义 user-service 和共享 runtime 的可观测性能力，覆盖健康检查、OpenAPI、metrics、tracing、日志、运行时故障处理和部署观测资产。

## Requirements

### Requirement: 健康检查与运行时诊断端点

系统 MUST 在业务 API 之外提供 `/livez`、`/readyz`、`/startupz`、配置化 metrics endpoint、OpenAPI 文档端点和可选 pprof 诊断监听，并保持这些端点的访问边界和启停语义明确。健康检查 MUST 只通过稳定 public contract 读取跨 feature 运行状态，MUST NOT 直接依赖 feature infrastructure concrete implementation。

#### Scenario: 存活、就绪与启动检查

- **WHEN** 调用 `/livez`
- **THEN** endpoint MUST 只证明进程可响应，并 MAY 在外部依赖异常时继续成功
- **WHEN** PostgreSQL、Redis、Casbin policy 或 policy watcher 等就绪依赖不可用
- **THEN** `/readyz` 或 `/startupz` MUST 失败并返回可定位且不含 secret、DSN、SQL、token、Cookie、stacktrace 的信息

#### Scenario: 运行时端点访问边界

- **WHEN** user-service 注册健康检查、OpenAPI 或 metrics 路由
- **THEN** 路由 MUST 位于 `/api/v1` 之外，MUST NOT 经过 RBAC 业务授权
- **AND** metrics 配置无效时路由注册 MUST 返回错误，而不是静默使用错误配置
- **WHEN** `server.http.enabled=false`
- **THEN** user-service MUST 不启动 HTTP 监听，依赖 HTTP 的健康检查、OpenAPI 和 metrics 路由 MUST 不对外暴露

#### Scenario: pprof 受控暴露

- **WHEN** pprof 未显式启用
- **THEN** 系统 MUST 不注册或启动 pprof listener
- **WHEN** pprof 显式启用
- **THEN** 系统 MUST 使用来自 `observability.pprof` 的地址启动独立诊断 listener，并默认限制在 loopback 或受控网络边界
- **AND** pprof listener MUST 与业务 Gin router 分离

#### Scenario: 健康检查依赖 public contract

- **WHEN** service-level provider 构造 Casbin policy 或 policy watcher 健康检查
- **THEN** provider MUST 依赖 permission feature 暴露的只读 health/status interface
- **AND** provider MUST NOT import permission infrastructure casbin、redis watcher 或其他 concrete implementation 包

### Requirement: 业务与运行时路由访问边界

系统 MUST 由 user-service composition root 统一维护 HTTP route 的访问层级，并通过明确的 route registrar contract 接入 feature 路由。route registrar MUST 按 public、authenticated 和 authorized 层级注册，MUST NOT 依赖 Fx value group 的 slice 顺序表达安全或冲突语义。

#### Scenario: 分层注册业务路由

- **WHEN** user-service 注册 `/api/v1` 路由
- **THEN** public auth route MUST 不经过普通 access token middleware
- **AND** authenticated auth route MUST 经过 token version validator 认证 middleware
- **AND** permission、role 和 user 业务 route MUST 先经过认证 middleware，再经过 RBAC authorizer middleware

#### Scenario: route registrar 和冲突语义

- **WHEN** 新 feature 需要挂载 `/api/v1` 业务路由
- **THEN** feature MUST 通过对应访问层级的 route registrar contract 接入 composition root
- **WHEN** route registrar 通过 Fx value group 注入
- **THEN** 注册逻辑 MUST NOT 假设 group slice 顺序稳定
- **AND** 如果存在 path 冲突、顺序或 middleware 层级要求，composition root MUST 使用显式编排或稳定排序规则表达该要求

#### Scenario: route graph 可验证

- **WHEN** 运行 route graph 测试或 route diff 诊断
- **THEN** 健康检查、OpenAPI、metrics、auth、permission、role 和 user route 的 path、method、访问层级和 route template MUST 可被稳定验证
- **AND** 必需认证或授权依赖缺失时系统 MUST 拒绝部分注册，而不是降级开放

### Requirement: OpenAPI 运行时文档契约

系统 MUST 暴露并生成与当前 user-service HTTP API 一致的 OpenAPI 3 文档，覆盖认证、用户、角色、权限、RBAC 保护接口和健康检查；运行时 Swagger UI MUST 使用 `github.com/swaggo/files/v2` 的 embedded `fs.FS`。

#### Scenario: 访问和生成文档

- **WHEN** 调用方访问 OpenAPI 路由
- **THEN** 系统 MUST 返回与当前 HTTP API 匹配的文档
- **WHEN** 执行 `make user-service-openapi-generate`
- **THEN** `user-service/docs/openapi.go`、`openapi.json` 和 `openapi.yaml` MUST 被同步更新

#### Scenario: 生成物 drift

- **WHEN** API 注解或路由行为变化但 OpenAPI 生成物未同步
- **THEN** 完整验证 MUST 通过重新生成和 `git diff --exit-code` 暴露 drift

#### Scenario: Swagger UI 依赖唯一

- **WHEN** user-service 注册 `/openapi/*any`
- **THEN** 静态资源 MUST 来自 `github.com/swaggo/files/v2`
- **AND** 生产代码 MUST NOT 保留 v1 import、旧 handler fallback、版本探测或双写兼容路径

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

### Requirement: 本地缓存指标契约

系统 MUST 为 `common/runtime/localcache` 导出低基数 `aegiscore_localcache_*` 指标，且稳定契约 MUST 只覆盖请求、回源、自动驱逐和 item 容量，并由 dashboard、alert 和真实 metrics load 校验消费当前契约。

#### Scenario: 请求和回源指标

- **WHEN** cache 命中、未命中、loader 成功或 loader 失败
- **THEN** 系统 MUST 分别通过 `aegiscore_localcache_requests_total{cache,result="hit|miss"}` 和 `aegiscore_localcache_loads_total{cache,result="success|error"}` 导出累计值
- **AND** success MUST 直接来自 `Stats.LoadSuccess`，MUST NOT 通过 load 总数减去 error 计算
- **AND** 标签 MUST 仅使用固定 cache 名与固定 result 枚举，MUST NOT 包含 raw key、身份标识或原始错误

#### Scenario: 自动驱逐和容量指标

- **WHEN** TTL 过期或达到最大 item 数使 cache 自动移除条目
- **THEN** 系统 MUST 通过 `aegiscore_localcache_evictions_total{cache}` 导出累计自动驱逐值
- **WHEN** collector 读取 cache 容量
- **THEN** 系统 MUST 通过 `aegiscore_localcache_capacity{cache}` 导出配置的最大 item 数
- **AND** 显式 `Delete` 或 `Clear` MUST NOT 计入 eviction

#### Scenario: 删除底层实现细节指标

- **WHEN** collector、dashboard、alert、runbook 或 metrics load 脚本消费本地缓存指标
- **THEN** 系统 MUST NOT 导出或查询 `aegiscore_localcache_singleflight_total` 与 `aegiscore_localcache_writes_total`
- **AND** shared、double-check、set-dropped 和 admission-rejected MUST NOT 作为稳定统计字段、event label 或兼容 PromQL 保留

#### Scenario: metrics 禁用和观测资产

- **WHEN** metrics provider 被禁用
- **THEN** localcache collector MUST 不注册，但 localcache MUST 继续维护可由 `Stats()` 读取的本地快照
- **WHEN** Grafana、Prometheus alert 或 metrics load 脚本消费本地缓存指标
- **THEN** 其 MUST 使用 requests、loads、evictions 和 capacity 四组当前 metric family 及 `cache`、`result` 固定标签
- **AND** 源 dashboard、provisioning JSON、alert、metrics load 校验和 runbook MUST 在同一变更中保持一致

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

### Requirement: Tracing 与依赖观测生命周期

系统 MUST 通过最小 OTLP 配置提供 OpenTelemetry tracing，并为 Redis 命令、Ent 查询和 HTTP 请求传播上下文。constructor MUST 返回稳定、非 nil、可被 instrumentation 安全引用的 tracing facade；禁用或尚未启动时其底层 MUST 为 no-op。启用 tracing 后，`OnStart` MUST 创建 exporter 与 SDK provider 并安装到底层 facade；`OnStop` 或启动 rollback MUST 关闭真实资源并恢复 no-op。

#### Scenario: tracing 启停和 constructor 阶段语义

- **WHEN** tracing 关闭或 Fx graph 在 `fx.New` constructor 阶段构造 tracing provider
- **THEN** provider MUST 可注入给 Redis、Gin、Ent 等依赖方，并提供非 nil no-op tracer provider
- **AND** constructor 阶段 MUST NOT 连接 OTLP exporter、启动 batch processor 或执行可能阻塞的 exporter 初始化
- **WHEN** tracing 开启且 Fx app 执行 `OnStart(ctx)`
- **THEN** provider MUST 使用服务名、环境和 OTLP endpoint 初始化 exporter 与 SDK provider
- **AND** exporter 初始化 MUST 使用 lifecycle 启动 context，受 Fx 启动预算、取消和超时控制
- **AND** lifecycle 停止时 provider MUST 使用停止 context 关闭 SDK provider 和 exporter 资源并恢复 no-op

#### Scenario: tracing 配置或 exporter 构造失败

- **WHEN** tracing 配置缺失服务名、环境、非法采样率，或启用 tracing 但缺少 OTLP endpoint
- **THEN** Fx graph MUST 返回明确构造错误，MUST NOT 延迟到 Redis、Gin、Ent 或 HTTP server 初始化时才暴露
- **WHEN** tracing 开启且 OTLP exporter 构造失败
- **THEN** `OnStart(ctx)` MUST 返回包含 `create OTLP tracing exporter` 语义的错误
- **AND** 返回错误 MUST 通过标准错误 wrapping 保留底层 gRPC、TLS、endpoint 或 context cause

#### Scenario: 后续启动失败不泄漏 tracing 资源

- **WHEN** tracing 已启用且 Fx App 在 tracing `OnStart` 成功后因后续 hook 失败而启动失败
- **THEN** App MUST 关闭 tracing `OnStart` 创建的 provider、batch processor 和 exporter
- **AND** 关闭错误 MUST 被保留或记录为可诊断信息，不得静默吞掉
- **AND** shutdown 后 tracing facade MUST 恢复到安全 no-op 状态，而不是悬挂已关闭 provider

#### Scenario: Redis 和 Ent 依赖观测

- **WHEN** user-service 执行 Redis 命令
- **THEN** 系统 MUST 创建低风险属性的 span 并传播服务 tracing provider
- **AND** span MUST NOT 记录完整 key、参数、token、密码或连接 secret
- **WHEN** Redis tracing instrumentation 返回错误
- **THEN** Redis client constructor MUST 返回包含 `instrument redis tracing` 的错误并关闭已创建 client，MUST NOT panic
- **WHEN** Ent 执行查询
- **THEN** 系统 MUST 产生 span，并记录低基数 latency 与 error metrics
- **AND** 观测 MUST NOT 修改 SQL、事务、schema、查询返回值或错误语义

#### Scenario: Redis metrics 探测取消

- **WHEN** metrics HTTP scrape context 被取消
- **THEN** Redis PING MUST 尽快终止
- **WHEN** collector 经标准 `Collect` 直接调用
- **THEN** MUST 使用 background context 与 collector timeout，不得声称感知 HTTP 取消
- **AND** 最小探测间隔、快照复用及 `aegiscore_redis_*` 指标契约 MUST 保持不变

### Requirement: 安全指标与部署观测资产

系统 MUST 维护 Prometheus alerts、Grafana dashboards、Compose 观测配置、生成脚本和 runbook，使 RBAC、认证安全及 runtime 关键指标具有可行动的观测视图且不会引入高基数。

#### Scenario: 业务安全和性能指标

- **WHEN** dashboard 展示 RBAC Enforce 性能
- **THEN** MUST 使用低基数 histogram 展示 P95 和 P99，并同步到源码与 Compose provisioning dashboard
- **WHEN** 一次性会话消费、重复消费拒绝、撤销投影或补偿失败
- **THEN** 系统 MUST 记录对应低基数指标
- **AND** alert 与 metrics load 校验 MUST 覆盖可导致安全撤销失效的信号并指向稳定 runbook

#### Scenario: 观测资产生成和 drift

- **WHEN** dashboard source 或生成逻辑变化
- **THEN** 生成脚本 MUST 更新 provisioning JSON
- **AND** `make compose-dashboard-check` 或等价校验 MUST 在生成物 drift 时失败

### Requirement: 运行时故障、初始化保护与优雅关闭

系统 MUST 将 HTTP 或 pprof listener 的非预期退出转换为 Fx shutdown signal，并在统一的 `runtime.lifecycle.stop_timeout` 总预算内按逆序 lifecycle hook 完成优雅关闭。可预期的资源、配置和依赖错误 MUST 优先通过 constructor 返回 `error` 暴露，MUST NOT 依赖 panic recovery 表达正常失败路径。

#### Scenario: listener 非预期退出

- **WHEN** HTTP 或 pprof `Serve` 在未进入正常关闭阶段时返回错误
- **THEN** 系统 MUST 记录可诊断错误并触发非零内部 shutdown signal
- **WHEN** 正常关闭导致 `http.ErrServerClosed`
- **THEN** 系统 MUST NOT 将其视为内部故障

#### Scenario: 外部与内部退出共用预算

- **WHEN** 外部终止信号或内部故障触发关闭
- **THEN** 系统 MUST 使用同一未被取消的上游 context value 和 `runtime.lifecycle.stop_timeout` 总预算执行 `App.Stop`
- **AND** 局部 HTTP、gRPC、tracing 或 logger timeout MUST NOT 替代总预算
- **WHEN** 前序 `OnStop` hook 已消耗部分总预算
- **THEN** 后续 hook MUST 只使用剩余时间，总关闭耗时 MUST NOT 因每个组件重新创建完整预算而无界增长

#### Scenario: 快速正常关闭和 pprof 强制关闭

- **WHEN** 所有 hook 在预算内完成
- **THEN** App MUST 立即完成关闭，不得等待完整 timeout
- **WHEN** pprof 已启用且 `OnStop` 调用 `server.Shutdown(ctx)` 返回错误
- **THEN** 系统 MUST 对同一个 pprof server 执行 best-effort `server.Close()`
- **AND** 返回错误 MUST 保留 `Shutdown` 失败信息，当 `Close` 也失败时 MUST 同时包含强制关闭失败信息
- **AND** 重复停止 MUST NOT panic 或阻塞

#### Scenario: Fx DI 初始化边界保护

- **WHEN** Fx 在 user-service composition root 中执行 constructor、decorator 或 Invoke 时发生未预期 panic
- **THEN** App 构造或启动 MUST 通过 Fx error 暴露 panic 信息
- **AND** 进程 MUST NOT 因该 DI 初始化 panic 直接崩溃
- **WHEN** HTTP handler、worker task、后台 goroutine 或 lifecycle hook 运行期发生 panic
- **THEN** `fx.RecoverFromPanics()` MUST NOT 被视为这些运行期边界的恢复策略
- **AND** 对应边界 MUST 使用其自身已有或显式设计的 panic 处理机制
