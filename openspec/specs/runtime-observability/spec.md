## Purpose

定义 user-service 和共享 runtime 的可观测性能力，覆盖运行时路由、健康检查、OpenAPI、metrics、tracing、日志、故障处理和部署观测资产。
## Requirements
### Requirement: HTTP 路由、健康检查与 OpenAPI

系统 MUST 在业务 API 之外提供 `/livez`、`/readyz`、`/startupz`、配置化 metrics endpoint、OpenAPI 文档端点和可选 pprof 诊断监听。user-service composition root MUST 显式集中挂载 auth、permission、role 和 user 路由，并按 public、authenticated 和 authorized 层级维护访问边界，MUST NOT 依赖 Fx value group 的 slice 顺序表达安全、冲突或必需路由语义。健康检查 MUST 只通过稳定 public contract 读取跨 feature 状态，MUST NOT 依赖 feature infrastructure concrete implementation。OpenAPI 3 文档 MUST 与当前 HTTP API 一致，运行时 Swagger UI MUST 使用 `github.com/swaggo/files/v2` 的 embedded `fs.FS`。

#### Scenario: 运行时端点与诊断边界

- **WHEN** 调用 `/livez`
- **THEN** endpoint MUST 只证明进程可响应，并 MAY 在外部依赖异常时继续成功
- **WHEN** PostgreSQL、Redis、Casbin policy 或 policy watcher 等就绪依赖不可用
- **THEN** `/readyz` 或 `/startupz` MUST 失败并返回可定位且不含 secret、DSN、SQL、token、Cookie、stacktrace 的信息
- **AND** Casbin policy 或 policy watcher 健康检查 MUST 依赖 permission feature 暴露的只读 health/status interface，MUST NOT import 其 infrastructure concrete implementation
- **WHEN** 注册健康检查、OpenAPI 或 metrics 路由
- **THEN** 路由 MUST 位于 `/api/v1` 之外且 MUST NOT 经过 RBAC 业务授权，metrics 配置无效时注册 MUST 返回错误
- **WHEN** `server.http.enabled=false`
- **THEN** 系统 MUST NOT 启动 HTTP listener，也 MUST NOT 暴露依赖 HTTP 的运行时端点
- **WHEN** pprof 未显式启用
- **THEN** 系统 MUST NOT 注册或启动 pprof listener
- **WHEN** pprof 显式启用
- **THEN** 系统 MUST 使用 `observability.pprof` 地址启动与业务 Gin router 分离的 listener，并默认限制在 loopback 或受控网络边界

#### Scenario: 业务路由装配与验证

- **WHEN** 注册 `/api/v1` 业务路由
- **THEN** public auth route MUST NOT 经过普通 access token middleware，authenticated auth route MUST 经过 token version validator，permission、role 和 user route MUST 再经过 RBAC authorizer
- **AND** composition root MUST 显式挂载 auth、permission、role 和 user 的 transport route 函数，MUST NOT 要求固定 feature 通过 feature-local registrar 或 Fx route value group 接入
- **AND** path 冲突、注册顺序和 middleware 层级 MUST 通过显式编排表达
- **WHEN** 运行 route graph 测试或 route diff 诊断
- **THEN** 健康检查、OpenAPI、metrics 及各 feature route 的 path、method、访问层级和 route template MUST 可稳定验证
- **AND** 必需认证、授权或 feature controller 缺失时系统 MUST 在构图或注册阶段失败，MUST NOT 降级开放或启动缺失路由的服务

#### Scenario: OpenAPI 文档、生成与 drift

- **WHEN** 调用方访问 OpenAPI 路由
- **THEN** 系统 MUST 返回覆盖认证、用户、角色、权限、RBAC 保护接口和健康检查且与当前 HTTP API 匹配的文档
- **WHEN** 执行 `make user-service-openapi-generate`
- **THEN** `user-service/docs/openapi.go`、`openapi.json` 和 `openapi.yaml` MUST 同步更新
- **AND** API 注解或路由变化产生的生成物 drift MUST 由重新生成和 `git diff --exit-code` 暴露
- **WHEN** 注册 `/openapi/*any`
- **THEN** 静态资源 MUST 来自 `github.com/swaggo/files/v2`，生产代码 MUST NOT 保留 v1 import、旧 handler fallback、版本探测或双写兼容路径

### Requirement: Metrics 与部署观测资产

系统 MUST 提供显式表达启用或禁用状态的非 nil Prometheus metrics provider。HTTP、runtime、scheduler、workerpool、SQL、Redis、localcache 和 feature metrics MUST 稳定、低基数且不泄露敏感数据；localcache 稳定指标契约 MUST 只覆盖请求、回源、容量驱逐和 item 容量。系统 MUST 维护 Prometheus alerts、Grafana dashboards、Compose 观测配置、生成脚本和 runbook。PostgreSQL 与 Redis 资源名 MUST 分别为 `primary_db` 和 `cache_redis`；metrics `service` label、tracing `service.name`、日志与健康响应 `service` 字段、dashboard 变量和 alert 表达式 MUST 统一使用 `aegiscore-user-service`，MUST NOT 保留旧 `aegiscore-user-services` label、查询或兼容资产。metrics Fx provider MUST 以可识别的能力名称由 composition root 显式装配。

#### Scenario: Metrics 启停、依赖与标签契约

- **WHEN** metrics 启用
- **THEN** 系统 MUST 注册配置化 endpoint 并导出已注册 collector
- **WHEN** metrics 禁用
- **THEN** 系统 MUST NOT 暴露 endpoint 或 collector，但 MUST 为正式依赖图提供非 nil no-op provider
- **AND** metrics、tracing 和 feature-local `Metrics` MUST 是非 optional 依赖，缺失时 MUST 构图失败；provider 公开名称 MUST 表达 metrics 能力语义，MUST NOT 以通用 `NewFxProvider` 作为主要入口
- **WHEN** 系统记录 metrics、健康检查或告警查询
- **THEN** label MUST NOT 包含用户、角色、权限、会话或 token ID、trace/span ID、raw path、IP、邮箱、用户名、SQL、Redis key 或原始错误
- **AND** 资源名、指标 label、健康检查名称、dashboard 和 alert MUST 一致，低基数 allowlist、HTTP label names 和 duration buckets 的顺序与数值 MUST 稳定且调用方不可修改
- **WHEN** metrics middleware 跳过 runtime endpoint 或其他配置化请求
- **THEN** 请求总数和耗时 MAY 不记录，但 in-flight gauge MUST 在请求结束后归零，MUST NOT 因删除共享 label value 破坏并发计数
- **WHEN** feature-local `Metrics` 需要空实现
- **THEN** 系统 MUST 通过统一生成入口维护匹配接口的 no-op；业务指标 MUST 留在所属 feature，`common/runtime/observability/metrics` MUST NOT 承载 user-service 业务语义

#### Scenario: Scheduler 指标契约

- **WHEN** scheduler job 被触发、开始、完成、失败、跳过或发生锁续租失败
- **THEN** 系统 MUST 通过 `aegiscore_scheduler_jobs_total` 记录固定 event `triggered|started|completed|failed|skipped|lock_renew_failed`
- **AND** skipped reason MUST 只使用 `local_overlap|global_concurrency_limit|lock_busy|lock_error`，无特定 reason 的事件 MUST 使用 `none`
- **AND** `scheduler_job` MUST 来自注册时固定低基数 job key，label MUST NOT 包含 cron spec、原始错误、panic、Redis key、lock owner token、身份标识或业务实体 ID
- **WHEN** 已开始的任务完成、返回 error、因续租失败结束或 panic
- **THEN** `aegiscore_scheduler_job_duration_seconds` MUST 仅观察从 started 到任务退出的 completed/failed duration，MUST NOT 包含 overlap、全局并发或锁等待时间
- **WHEN** scheduler lock 续租失败
- **THEN** 系统 MUST 保留 `lock_renew_failed` counter event、Prometheus alert、Grafana 查询和 runbook 定位，不得因内部重构丢失该观测信号
- **WHEN** metrics provider 禁用
- **THEN** scheduler MUST 获得非 nil no-op metrics 实现，scheduler collector MUST NOT 注册

#### Scenario: 本地缓存指标契约

- **WHEN** cache 命中、未命中、loader 成功或失败
- **THEN** 系统 MUST 分别通过 `aegiscore_localcache_requests_total{cache,result="hit|miss"}` 和 `aegiscore_localcache_loads_total{cache,result="success|error"}` 导出累计值，success MUST 直接来自 `Stats.LoadSuccess`，MUST NOT 由 load 总数减去 error 推导
- **WHEN** 达到最大 item 数发生容量驱逐，或 collector 读取容量
- **THEN** 系统 MUST 通过 `aegiscore_localcache_capacity_evictions_total{cache}` 和 `aegiscore_localcache_capacity{cache}` 导出累计容量驱逐值与配置容量
- **WHEN** item 因 TTL 到期、`Invalidate` 或 `InvalidateAll` 被移除
- **THEN** `aegiscore_localcache_capacity_evictions_total` MUST NOT 增加，系统 MUST NOT 把这些移除解释为容量压力
- **AND** 标签 MUST 仅使用固定 cache 名和 result 枚举，MUST NOT 包含 raw key、身份标识或原始错误
- **AND** 系统 MUST NOT 导出或查询 `aegiscore_localcache_evictions_total`、`aegiscore_localcache_singleflight_total` 或 `aegiscore_localcache_writes_total`，MUST NOT 将 shared、double-check、set-dropped、admission-rejected、explicit invalidation 或 TTL expiration 保留为稳定统计字段、event label 或兼容 PromQL
- **WHEN** metrics provider 禁用
- **THEN** localcache collector MUST NOT 注册，但 localcache MUST 继续维护可由 `Stats()` 读取的快照
- **AND** dashboard、alert、metrics load 校验和 runbook MUST 仅消费 requests、loads、capacity evictions、capacity 及固定 `cache`、`result` 标签，并与源 dashboard 和 provisioning JSON 在同一变更中保持一致

#### Scenario: 依赖探测、安全指标与资产 drift

- **WHEN** metrics HTTP scrape context 被取消
- **THEN** Redis PING MUST 尽快终止；标准 `Collect` 直接调用 MUST 使用 background context 与 collector timeout，MUST NOT 声称感知 HTTP 取消
- **AND** 最小探测间隔、快照复用及 `aegiscore_redis_*` 指标契约 MUST 保持不变
- **WHEN** dashboard 展示 RBAC Enforce 性能
- **THEN** MUST 使用低基数 histogram 展示 P95 和 P99，并同步到源码与 Compose provisioning dashboard
- **WHEN** 一次性会话消费、重复消费拒绝、撤销投影或补偿失败
- **THEN** 系统 MUST 记录对应低基数指标，alert 与 metrics load 校验 MUST 覆盖影响安全撤销的信号并指向稳定 runbook
- **WHEN** Prometheus rules、Grafana dashboard、Compose scrape config、观测文档、dashboard source 或生成逻辑变化
- **THEN** 查询、变量、静态 label、告警 label、dashboard UID、rule group、job name 和示例 MUST 使用单数服务名，生成脚本 MUST 更新 provisioning JSON
- **AND** `make compose-dashboard-check` 或等价校验 MUST 在生成物 drift 时失败

### Requirement: RBAC watcher 新鲜度健康与自恢复观测

user-service MUST 通过 permission application 的结构化只读 watcher status 判定 RBAC watcher 健康，并以最后一次成功 PostgreSQL revision 权威校准时间计算 staleness。公共健康响应、metrics、alerts 和 dashboard MUST 区分 stopped、reconnecting、recovered 与 stale 状态，MUST NOT 继续以任意历史 `LastError` 作为永久失败条件。

#### Scenario: 首次校准后进入就绪

- **WHEN** watcher 已运行但尚未成功完成一次 PostgreSQL revision 权威校准
- **THEN** watcher startup 和 readiness 检查 MUST 返回 unavailable 且使用稳定、不含底层错误的定位信息
- **WHEN** 首次权威校准成功且 Casbin projection ready
- **THEN** watcher startup 和 readiness 检查 MUST 恢复 available，并以该成功时间开始计算 staleness

#### Scenario: 新鲜窗口内订阅重连不制造粘滞失败

- **WHEN** watcher 正在运行且最后权威校准年龄不大于 `max_staleness`，但 subscription state 为 `reconnecting` 或保留历史失败时间
- **THEN** watcher 自身 readiness 检查 MUST 保持 available，订阅降级 MUST 通过结构化状态、metrics 和日志保持可见
- **AND** 独立 Redis health checker MAY 因 Redis 整体不可用使聚合 readiness 失败，但 watcher 检查 MUST NOT 因已恢复的历史错误永久失败

#### Scenario: 停止、从未校准或校准过期时拒绝流量

- **WHEN** watcher 未运行、循环意外退出、从未成功权威校准，或当前时间减去最后权威校准成功时间大于 `max_staleness`
- **THEN** watcher readiness MUST 返回 unavailable，且 `/readyz` MUST 返回 `503`
- **AND** 健康响应 MUST 只返回稳定的 stopped、not synchronized 或 stale 定位信息，MUST NOT 暴露原始 Redis/PostgreSQL 错误、地址、key、SQL、stacktrace 或 secret
- **AND** `/livez` MUST 继续只表达进程存活，不得因 watcher stale 而失败

#### Scenario: watcher 专用低基数指标

- **WHEN** metrics provider 启用并采集 watcher 状态
- **THEN** 系统 MUST 暴露 watcher running、subscription connected、最后订阅成功 timestamp、最后权威校准成功 timestamp、当前 reconcile staleness 和重连尝试计数
- **AND** 指标 label MUST 只使用固定 state、result 和 reason 枚举，MUST NOT 包含原始错误、Redis key、revision、event、user、role、permission 或其他高基数字段
- **AND** 系统 MUST 停止为 watcher 输出或查询 `aegiscore_runtime_component_running{resource="rbac_policy_watcher"}` 与 `aegiscore_runtime_component_last_error{resource="rbac_policy_watcher"}`，MUST NOT 双写旧指标

#### Scenario: 告警与 dashboard 表达当前风险

- **WHEN** watcher 停止或 reconcile staleness 持续超过配置预算
- **THEN** Prometheus MUST 产生可定位到实例的 critical 告警，Grafana MUST 展示 running、subscription state、最后校准成功时间和 staleness
- **WHEN** watcher 持续重连但权威校准仍在新鲜窗口内
- **THEN** Prometheus MUST 将其表达为 subscription degraded warning，MUST NOT 将单次历史错误持续表达为 watcher 不健康
- **AND** Compose dashboard MUST 由通用 dashboard 资产生成并通过 drift 检查保持一致

#### Scenario: 恢复状态可观察

- **WHEN** 初始订阅失败、Receive 错误或 revision 校准失败随后恢复
- **THEN** 当前错误和健康状态 MUST 在对应成功动作后恢复，最后成功 timestamp MUST 推进，staleness MUST 回落
- **AND** 失败与恢复日志 MUST 使用英文 message 和稳定 `snake_case` 字段，公共健康响应和 metrics label MUST NOT 包含底层 cause

### Requirement: 结构化日志与请求关联

系统 MUST 为每个 HTTP 请求建立 request ID，并通过共享 logger context 关联 access log、应用日志、有效 tracing context 和 Fx 初始化事件。日志 MUST 结构化输出到 stdout/stderr、保持分级、使用 `aegiscore-user-service` 服务名且不得记录敏感信息。

#### Scenario: 请求关联、错误与敏感信息

- **WHEN** 入站 `X-Request-ID` 合法
- **THEN** 系统 MUST 在响应头和请求日志中透传相同值
- **WHEN** header 缺失、空白、超长或含控制字符
- **THEN** 系统 MUST 生成新值，并由 `common/runtime/logger` 写入和读取以用于响应头、access log 和应用日志
- **WHEN** 请求具有有效 W3C trace context
- **THEN** 日志 MUST 包含独立的 `request_id`、`trace_id` 和 `span_id`；span context 无效时 MUST 省略 trace 字段但保留 request ID，metrics label MUST NOT 包含这些 ID
- **WHEN** 请求完成、发生 panic 或 span 对应操作失败
- **THEN** access log MUST 记录稳定字段，recovery MUST 记录错误并返回统一响应，span MUST 标记错误
- **AND** 日志 MUST NOT 包含密码、token、Cookie、Authorization、DSN、SQL 参数或完整 Redis key

#### Scenario: App logger 与 Fx event 生命周期

- **WHEN** 正式 App 启停或多个 App 并行运行
- **THEN** App MUST 使用显式注入的 logger 并在 Stop 时同步自身 logger，MUST NOT 安装或依赖进程级默认 logger；相关测试 MUST 隔离并恢复进程状态
- **WHEN** `AppOptions` 或 `NewApp` 构建正式 Fx App
- **THEN** Fx event logger MUST 由注入的 `*zap.Logger` 构造并进入统一结构化日志链路
- **AND** 常规构图、执行前后、module trace 或 lifecycle 事件 MUST 使用 debug 级别，构造、Invoke、rollback 或 lifecycle 失败 MUST 使用 error 级别
- **AND** `LogEvent` MUST NOT 执行网络 I/O、远程导出、阻塞式重试或业务副作用，Fx event logger MUST NOT 替换进程级默认 logger 或引入额外同步生命周期

### Requirement: Tracing 与依赖观测生命周期

系统 MUST 通过最小 OTLP 配置提供 OpenTelemetry tracing，并为 HTTP、Redis 和 Ent 传播上下文。constructor MUST 返回稳定、非 nil 且可由 instrumentation 安全引用的 tracing facade；禁用或尚未启动时 MUST 使用 no-op，启用后 MUST 在 lifecycle 内安装和关闭真实资源并恢复 no-op。tracing Fx provider MUST 以可识别的能力名称由 composition root 显式装配。Ent 观测能力 MUST 通过显式插件配置启用；SQL log、tracing 和 metrics MUST 能独立安装，且默认仅启用 Ent tracing 插件。

#### Scenario: Tracing 启停、失败与回滚

- **WHEN** tracing 关闭或处于 `fx.New` constructor 阶段
- **THEN** provider MUST 可注入 Redis、Gin、Ent 等依赖方并提供非 nil no-op tracer provider，MUST NOT 连接 exporter、启动 batch processor 或执行可能阻塞的初始化
- **AND** provider 公开名称 MUST 表达 tracing 能力语义，MUST NOT 以通用 `NewFxProvider` 作为主要入口
- **WHEN** tracing 配置缺失服务名、环境、合法采样率或启用时缺少 OTLP endpoint
- **THEN** Fx graph MUST 返回明确构造错误，MUST NOT 延迟到依赖或 server 初始化
- **WHEN** tracing 启用且执行 `OnStart(ctx)`
- **THEN** provider MUST 使用服务名、环境和 OTLP endpoint 在启动 context 预算内初始化 exporter 与 SDK provider；构造失败 MUST 返回包含 `create OTLP tracing exporter` 且通过标准 wrapping 保留底层 cause 的错误
- **WHEN** lifecycle 停止或后续 hook 失败触发 rollback
- **THEN** 系统 MUST 使用停止 context 关闭 provider、batch processor 和 exporter 并恢复 no-op，关闭错误 MUST 被保留或记录，MUST NOT 悬挂已关闭 provider

#### Scenario: Redis 观测

- **WHEN** user-service 执行 Redis 命令
- **THEN** 系统 MUST 创建仅含低风险属性并传播服务 tracing provider 的 span，MUST NOT 记录完整 key、参数、token、密码或连接 secret
- **WHEN** Redis tracing instrumentation 返回错误
- **THEN** constructor MUST 返回包含 `instrument redis tracing` 的错误并关闭已创建 client，MUST NOT panic

#### Scenario: 默认仅启用 Ent tracing

- **WHEN** user-service 使用默认配置创建 Ent client
- **THEN** 系统 SHALL 安装 Ent tracing 插件
- **AND** 系统 SHALL NOT 安装 SQL log driver 插件
- **AND** 系统 SHALL NOT 注册 Ent query metrics
- **WHEN** Ent 执行 query 或 mutation
- **THEN** 系统 SHALL 产生 Ent span，MUST NOT 改变 SQL、事务、schema、查询返回值或错误语义

#### Scenario: 显式启用 SQL log 插件

- **WHEN** 配置 `ent.plugins.sql_log.enabled=true`
- **THEN** 系统 SHALL 使用 SQL log driver plugin 包装 Ent driver
- **AND** 慢 SQL、SQL error 和 debug SQL 行为 SHALL 由该插件负责
- **AND** `ent.plugins.sql_log.debug` SHALL 控制是否记录成功 SQL 的 debug 日志
- **AND** `ent.plugins.sql_log.slow_threshold` SHALL 控制慢 SQL 阈值

#### Scenario: 显式启用 Ent metrics 插件

- **WHEN** 配置 `ent.plugins.metrics.enabled=true`
- **AND** metrics provider 已启用
- **THEN** 系统 SHALL 注册 Ent query latency 和 error metrics
- **AND** Ent query metrics 名称 SHALL 保持为 `aegiscore_user_service_ent_query_duration_seconds` 和 `aegiscore_user_service_ent_query_errors_total`
- **WHEN** metrics provider 为空或禁用
- **THEN** 系统 SHALL NOT 注册 Ent query metrics，MUST NOT panic
- **WHEN** Ent metrics collector 注册失败
- **THEN** Ent client 创建 SHALL 失败并向上传播注册错误

#### Scenario: Ent tracing 不依赖 SQL log 插件

- **WHEN** 配置 `ent.plugins.tracing.enabled=true`
- **AND** 配置 `ent.plugins.sql_log.enabled=false`
- **WHEN** Ent query 或 mutation 执行
- **THEN** 系统 SHALL 记录 Ent span
- **AND** 系统 SHALL NOT 输出 Ent SQL log

#### Scenario: Ent 观测插件配置契约

- **WHEN** user-service 读取 Ent 观测配置
- **THEN** 系统 MUST 使用 `ent.plugins.sql_log.enabled`、`ent.plugins.sql_log.debug`、`ent.plugins.sql_log.slow_threshold`、`ent.plugins.tracing.enabled` 和 `ent.plugins.metrics.enabled` 表达插件启停和 SQL log 行为
- **AND** 系统 MUST NOT 使用 `ent.sql_debug` 作为配置契约

### Requirement: 运行时故障、诊断与依赖观测边界

系统 MUST 将 HTTP 或 pprof listener 的非预期退出转换为 Fx shutdown signal，并在统一预算内优雅关闭。业务 HTTP 与 pprof MUST 使用 `common/runtime/httpserver` 的独立 managed server，composition root MUST 显式绑定 runtime server、诊断监听及 Redis/PostgreSQL 观测依赖；可预期错误 MUST 通过 constructor 返回，依赖健康、metrics、tracing 与日志 MUST 保持低基数且不泄露敏感信息。

#### Scenario: Listener 故障与关闭预算

- **WHEN** 业务 HTTP 或 pprof 启用且 Fx 执行 OnStart
- **THEN** hook MUST 直接调用对应 `Managed.Start(ctx)`，监听失败 MUST 同步阻断 App 启动且 MUST NOT 留下后台资源
- **WHEN** HTTP 或 pprof `Serve` 在正常关闭前返回非预期错误
- **THEN** 服务侧 `OnServeError` MUST 记录可诊断错误并触发 exit code 1 的内部 shutdown signal；`http.ErrServerClosed`、`net.ErrClosed` 与停止期间的 context cancellation MUST NOT 被视为内部故障
- **WHEN** 外部信号或内部故障触发关闭
- **THEN** hook MUST 直接调用对应 `Managed.Stop(ctx)`，系统 MUST 使用同一未被取消的上游 context value 和 `runtime.lifecycle.stop_timeout` 总预算执行 `App.Stop`，各 Managed 的内部 shutdown timeout MUST 不大于总预算，后续 hook MUST 仅使用剩余时间，MUST NOT 通过为每个组件重建完整总预算使总耗时无界增长
- **WHEN** 某次 Fx Stop 等待 context 先于 Managed cleanup 到期
- **THEN** 本次等待 MUST 返回 context error，Managed 后台 cleanup MUST 继续，后续 Stop MUST 能继续等待同一 cleanup
- **WHEN** 所有 hook 在预算内完成
- **THEN** App MUST 立即关闭，MUST NOT 等待完整 timeout
- **WHEN** 业务 HTTP 或 pprof 的优雅关闭失败
- **THEN** 对应 Managed MUST 对同一 server best-effort `Close()`、等待 handler 与 `Serve` goroutine，并保留 Shutdown、Close、drain 与 Serve 的最终错误；重复停止 MUST NOT panic 或阻塞

#### Scenario: DI 初始化与 composition root 绑定

- **WHEN** Fx constructor、decorator 或 Invoke 发生未预期 panic
- **THEN** App 构造或启动 MUST 通过 Fx error 暴露信息，进程 MUST NOT 直接崩溃
- **AND** `fx.RecoverFromPanics()` MUST NOT 被视为 HTTP handler、worker、后台 goroutine或 lifecycle hook 运行期 panic 的恢复策略，各边界 MUST 使用自身机制
- **WHEN** 构建正式或测试 Fx App
- **THEN** composition root MUST 显式绑定 process runtime 初始化、metrics、tracing、服务资源、feature lifecycle 和 runtime server，process runtime 初始化 MUST 先于 server 启动
- **AND** `common/runtime/httpserver` 与 `common/runtime/observability` MUST 保持业务中立，MUST NOT 导入 user-service feature、router、bootstrap、Gin、Fx 或服务私有配置包
- **WHEN** `server.http.enabled=false` 或 pprof 未启用
- **THEN** 对应 composition DTO MUST 显式表达 disabled，MUST NOT 构造或启动 `Managed`，也 MUST NOT 注册对应 lifecycle hook
- **WHEN** 业务 HTTP 与 pprof 同时启用
- **THEN** composition MUST 从各自配置映射地址和 handler，并构造两个不同的 `Managed` 实例；pprof shutdown timeout MUST 由 composition 显式选择，核心包 MUST NOT 回退到业务默认值
- **WHEN** 正式 `AppModule` 构建 runtime graph
- **THEN** composition root MUST 通过具名注册函数或等价可识别结构显式解析业务 HTTP 与 pprof runtime DTO，MUST NOT 依赖空匿名 Invoke
- **AND** bootstrap 测试 MUST 验证两个 server 及 lifecycle hook 注册链路仍存在，bootstrap MUST NOT 保留通用 listener、Serve、Shutdown、Close 或 drain 状态机

#### Scenario: Compose 默认不暴露 pprof

- **WHEN** 调用方渲染默认 Compose 配置
- **THEN** user-service 环境变量 MUST NOT 设置 `AEGISCORE_OBSERVABILITY_PPROF_ENABLED=true`
- **AND** user-service 环境变量 MUST NOT 设置 `AEGISCORE_OBSERVABILITY_PPROF_ADDR=0.0.0.0:6060`
- **AND** user-service ports MUST NOT 包含 `6060:6060`

#### Scenario: Redis command filter 语义

- **WHEN** Redis command 为 `AUTH`
- **THEN** command filter MUST 返回 true 表示过滤该命令且不生成 span
- **WHEN** Redis command 为 `HELLO ... AUTH ...`
- **THEN** command filter MUST 返回 true 表示过滤该命令且不生成 span
- **WHEN** Redis command 为 `PING`
- **THEN** command filter MUST 返回 true 表示过滤该命令且不生成 span
- **WHEN** Redis command 为普通业务命令
- **THEN** command filter MUST 返回 false 表示允许生成 span

#### Scenario: Cluster PING 健康检查

- **WHEN** `/readyz` 或 `/startupz` 检查 `redis.cache_redis`
- **THEN** health checker MUST 通过 Cluster-capable pinger 执行 PING
- **AND** Redis Cluster 不可用时响应 MUST 只返回稳定不可用消息，不得包含 endpoint、密码、key、slot 或底层错误文本

#### Scenario: Redis ping metrics 保持低基数

- **WHEN** metrics scrape 触发 Redis ping collector
- **THEN** collector MUST 支持 Cluster client 并继续导出既有 `aegiscore_redis_*` 指标契约
- **AND** 指标 label MUST 只使用稳定 resource 等低基数字段，MUST NOT 增加 node、addr、slot、mode 或错误文本 label

#### Scenario: Cluster Redis 命令 tracing

- **WHEN** user-service 通过 Redis Cluster client 执行 Redis 命令
- **THEN** tracing MUST 使用服务注入的 tracer provider 创建低风险 span
- **AND** span MUST NOT 记录完整 key、参数、token、密码、seed endpoint 或连接 secret

#### Scenario: instrumentation 失败清理

- **WHEN** Redis Cluster tracing instrumentation 返回错误
- **THEN** constructor MUST 返回包含 `instrument redis tracing` 的错误并关闭已创建 client
- **AND** 系统 MUST NOT panic 或留下未关闭 Redis client

#### Scenario: Access log 记录真实客户端地址

- **WHEN** 请求来自已配置的 trusted proxy，且 forwarded headers 已由入口层清洗
- **THEN** HTTP access log 的 `client_ip` 字段 MUST 记录 Gin 解析后的真实客户端地址
- **AND** 日志字段 MUST NOT 额外记录完整 forwarded header 链路或未清洗原始 header

#### Scenario: 未信任代理时忽略 forwarded headers

- **WHEN** 请求来自未受信任 TCP peer
- **THEN** HTTP access log 和认证失败日志的 `client_ip` 字段 MUST 记录 TCP peer 地址
- **AND** 请求携带的 `X-Forwarded-For` 或 `X-Real-IP` MUST NOT 改变该字段

### Requirement: Gin 入站 HTTP 观测路径低基数契约

系统 MUST 对 Gin 入站 HTTP 观测路径使用低基数 route template。access log、认证失败日志、绑定或校验失败日志、HTTP metrics route label、trace span name、授权 object 和默认观测过滤判断 MUST 使用 Gin route template 或固定 `__unmatched__`，MUST NOT 发出或依赖原始 URL path。

#### Scenario: 匹配动态业务路由的观测路径
- **WHEN** Gin 入站请求匹配包含路径参数的业务 route template，例如 `/api/v1/users/:user_id`
- **THEN** 日志 `path` 字段、HTTP metrics route label、trace span name 和授权 object MUST 使用该 route template
- **AND** 系统 MUST NOT 在这些默认观测字段中写入真实用户 ID、角色 ID、权限 ID、session ID、tenant、cursor、UUID 或其他 raw path 参数值

#### Scenario: 未匹配路由的观测路径
- **WHEN** Gin 入站请求未匹配任何 route template
- **THEN** 日志 `path` 字段、HTTP metrics route label、trace span name 和授权 object MUST 使用固定值 `__unmatched__`
- **AND** 系统 MUST NOT 回退到 `c.Request.URL.Path`、`request.URL.Path` 或等价 raw URL path

#### Scenario: 请求绑定和校验失败日志
- **WHEN** Gin 入站请求在 binding 或 validation 阶段失败
- **THEN** 失败日志 MUST 使用匹配 route template 或 `__unmatched__` 作为 `path` 字段
- **AND** 失败日志 MUST NOT 因请求体绑定失败、参数校验失败或上下文尚未进入 feature controller 而记录 raw URL path

#### Scenario: runtime endpoint 观测跳过判断
- **WHEN** 系统判断 `/metrics`、`/livez`、`/readyz` 或 `/startupz` 等 runtime endpoint 是否跳过成功请求日志、请求计数或请求耗时
- **THEN** 判断输入 MUST 是 route template 或显式静态配置归一化结果
- **AND** Gin 入站观测跳过判断 MUST NOT 使用 raw URL path

#### Scenario: tracing 过滤与 span name
- **WHEN** OTel Gin instrumentation 处理入站 HTTP 请求
- **THEN** 应用内 Gin tracing 逻辑 MUST NOT 在 route match 前基于 `request.URL.Path` 过滤请求
- **AND** HTTP server span name MUST 使用 `METHOD <route template>` 或 `METHOD __unmatched__`
- **AND** 低噪声 tracing 过滤如仍需要，MUST 在应用外或基于稳定 route/span name 执行，MUST NOT 依赖 raw URL path

#### Scenario: HTTP metrics route fallback
- **WHEN** HTTP metrics middleware 记录未匹配 Gin 入站请求
- **THEN** route label fallback MUST 固定为 `__unmatched__`
- **AND** 公共 middleware 配置 MUST NOT 允许调用方提供可能包含 raw path 或高基数值的 route fallback

### Requirement: RBAC 同步可观测性、健康与投影 lag

系统 MUST 为 outbox dispatcher 与 policy watcher 暴露低基数 metrics、结构化日志和只读 status，并接入 health/readiness。Policy reload lag MUST 只表示 PostgreSQL latest policy revision 与 Casbin engine actual applied revision 的非负差值；user-role revision、Redis counter、Pub/Sub payload 或 reload attempt MUST NOT 充当 policy lag 权威值。健康探测 MUST 只读，MUST NOT 修改 outbox event。

#### Scenario: backlog、lag 与投递指标

- **WHEN** dispatcher claim、publish、ack、失败、重试或采集 outbox 状态
- **THEN** feature metrics MUST 记录固定 result/reason/kind 枚举下的处理计数、due backlog、最老未完成 event age 和 loop 运行状态
- **AND** metrics label MUST NOT 包含 event/revision/user/role/permission ID、idempotency key、原始错误、SQL、Redis key、payload 或其他高基数字段
- **AND** `kind` MUST 使用固定枚举区分 policy 事件与 user-role 事件，MUST NOT 通过 label 暴露具体 revision 或用户标识

#### Scenario: dispatcher 结构化日志

- **WHEN** event 被 claim、成功投递、失败退避、lease 冲突或循环状态变化
- **THEN** 日志 MUST 使用英文 message 和稳定 `snake_case` 字段，并 MAY 记录 policy revision、user-role revision、attempt、kind、reason 和稳定错误类别
- **AND** 日志 MUST NOT 记录完整 event payload、SQL、Redis key、连接 secret 或将原始底层错误暴露到公共健康响应
- **AND** policy 事件字段 MUST 使用 `policy_revision`，user-role 事件字段 MUST 使用 `user_role_revision`，MUST NOT 用 `policy_revision` 表示用户角色绑定提交水位

#### Scenario: 只读 health 与 readiness

- **WHEN** dispatcher 正在运行且可读取 outbox 状态
- **THEN** status MUST 报告最近成功时间、最近错误类别、due count 和最老未完成 event age，探测 MUST NOT 改变任何 event
- **WHEN** dispatcher 未启动、循环意外退出或 outbox 状态查询失败
- **THEN** readiness MUST 失败并返回稳定且不含敏感信息的定位结果
- **AND** 单次 publish 失败或处于退避中的 backlog MUST 保持可见且不得终止 dispatcher 循环

#### Scenario: metrics 禁用时保持行为

- **WHEN** 全局 metrics provider 禁用
- **THEN** dispatcher MUST 继续 claim、发布、重试和更新只读 status，并通过非 nil no-op feature metrics 满足正式依赖图
- **AND** 系统 MUST NOT 因 collector 未注册而改变 event 投递、health 或 readiness 状态机

#### Scenario: 数据库 latest policy 超前时暴露非零 lag

- **WHEN** watcher 成功读取的 database latest policy revision 大于 local applied projection revision
- **THEN** `aegiscore_user_service_rbac_policy_reload_lag` MUST 记录两者的正差值
- **AND** watcher MUST 记录 database revision mismatch 事件，metrics label MUST 只使用固定低基数 source、result 和 reason allowlist
- **AND** dashboard、alert 和 runbook MUST 将该值解释为数据库 policy 授权事实与本地实际 Casbin 投影之间的差值

#### Scenario: user-role revision 禁止影响 policy lag

- **WHEN** 只有用户角色绑定变化且 latest user-role revision 高于 local applied policy revision
- **THEN** `aegiscore_user_service_rbac_policy_reload_lag` MUST NOT 因 user-role revision 变为非零
- **AND** watcher MUST NOT 将 user-role revision mismatch 记录为 policy reload lag，MUST NOT 触发 policy reload failure 或 policy readiness 失败
- **AND** user-role 通知 MAY 通过固定 kind/reason 的 dispatcher 或 watcher 计数、缓存失效计数和结构化日志体现

#### Scenario: lag 为零禁止假收敛

- **WHEN** watcher 基于一次成功数据库 policy revision 读取记录 lag 为 `0`
- **THEN** local applied projection revision MUST 大于或等于该次读取的 database latest policy revision
- **AND** Redis counter 缺失、落后、重建、等于 local 值、user-role revision 推进或 Pub/Sub 消息处理成功 MUST NOT 单独使 policy lag 变为 `0`
- **WHEN** local applied revision 高于本次读取的 database latest policy revision
- **THEN** lag MUST 按非负规则记录为 `0` 且 MUST NOT 降低 local applied revision

#### Scenario: 查询或 reload 失败不清零 lag

- **WHEN** database latest policy revision 读取失败
- **THEN** 系统 MUST 记录固定 `revision_store_unavailable` 或等价 reason，并保留上一 lag 观测值，MUST NOT 用 Redis、hint revision 或 user-role revision 更新 lag
- **WHEN** database latest policy revision 读取成功但 reload 失败或实际 applied revision 仍低于目标
- **THEN** 系统 MUST 记录固定 `reload_failed` reason 并保留基于 database latest policy revision 与 actual applied 计算的非零 lag
- **AND** 只有后续成功数据库 policy 校准证明 actual applied revision 不低于 database latest policy revision 时，系统才 MUST 把 lag 记录为 `0`

#### Scenario: watcher 指标 reason 与日志字段

- **WHEN** watcher 记录周期检查、Pub/Sub 唤醒、revision mismatch、reload success 或 reload failure
- **THEN** metrics MUST 使用稳定低基数 source/result/reason 区分 `revision_store_unavailable`、`revision_mismatch`、`reload_failed` 与成功
- **AND** metrics reason MUST NOT 继续以 Redis version store 不可用表达数据库 revision 查询失败，也 MUST NOT 包含 revision 数值、用户、角色、权限、Redis key 或原始错误文本
- **AND** 结构化日志 MUST 使用 `database_latest_policy_revision`、`local_applied_policy_revision`、`target_policy_revision`、`hint_policy_revision`、`user_role_revision`、`source` 和稳定 reason 中的适用字段
- **AND** 日志 MUST NOT 使用含混的 `remote_policy_revision` 或 `remote_version` 字段把 Redis 消息、user-role revision 或 counter 描述为数据库 policy 权威事实，也 MUST NOT 记录 policy 内容、SQL、Redis key 或 secret

#### Scenario: dashboard、alert 与 fixture 同步

- **WHEN** Grafana dashboard 展示 RBAC policy reload lag 或 Prometheus alert 评估持续未收敛
- **THEN** 查询、panel 说明、alert annotation 和 runbook MUST 使用 database latest policy revision 与 local applied projection revision 语义
- **AND** alert MUST 继续覆盖超过既定最终收敛 SLO 的非零 policy lag，并将 policy revision store unavailable 与 policy reload failure 作为可定位关联信号
- **AND** dashboard 源、Compose provisioning 副本、Prometheus rules、metrics load 测试和相关 fixture MUST 在同一 change 中更新
- **AND** 生成或检查命令 MUST 在旧 Redis/local version 文案、混合 revision 文案、PromQL 或 dashboard drift 存在时失败

### Requirement: Metrics provider registry 与 scrape context 契约

系统 MUST 通过 `common/runtime/observability/metrics` 提供显式 enabled/disabled 状态的非 nil provider，并为启用状态使用独立 Prometheus registry。provider MUST 支持重复注册幂等、context-aware gather 和基于 HTTP request context 的 scrape handler；禁用状态 MUST 保持 no-op 且不得暴露 collector 或 HTTP metrics 输出。

#### Scenario: 启用 provider 使用独立 registry

- **WHEN** metrics provider 基于启用配置创建
- **THEN** provider MUST 返回 `Enabled()=true`，并使用独立 registry、registerer 和 gatherer
- **AND** provider MUST NOT 注册或依赖 Prometheus global registry
- **AND** service 与 environment label MUST 继续由 provider registerer 统一包装为稳定低基数字段

#### Scenario: 禁用 provider 保持正式依赖图可用

- **WHEN** metrics provider 基于禁用配置创建
- **THEN** provider MUST 返回非 nil provider 且 `Enabled()=false`
- **AND** `Registerer()` 与 `Gatherer()` MUST 返回 nil，`Register` 和 `MustRegister` MUST 保持 no-op
- **AND** `HTTPHandler` MUST NOT 暴露 metrics 内容

#### Scenario: 重复注册不破坏启动

- **WHEN** 同一 collector 或等价 collector 被重复注册到启用 provider
- **THEN** provider MUST 将 Prometheus `AlreadyRegisteredError` 视为成功
- **AND** 其他注册错误 MUST 继续向调用方返回，nil collector MUST 返回稳定错误

#### Scenario: HTTP scrape context 传播给 context-aware collector

- **WHEN** 调用方通过 `HTTPHandler` 暴露 metrics endpoint 且 HTTP request context 被取消
- **THEN** provider MUST 通过 `GatherContext` 将该 request context 提供给实现 `ContextCollector` 的 collector
- **AND** Redis PING 等支持 context 的 collector MUST 能在 scrape 取消时尽快终止
- **AND** 标准 `Collect` 或 `Gatherer().Gather()` 直接调用 MUST 使用 background context，MUST NOT 声称感知 HTTP request cancellation

#### Scenario: metrics label 保持低基数

- **WHEN** runtime collector、feature metrics 或自定义 collector 通过 provider 注册并导出指标
- **THEN** label MUST 只使用固定资源名、结果、状态、reason 或 service/environment 等低基数字段
- **AND** label MUST NOT 包含用户、角色、权限、会话、token、trace/span ID、raw path、IP、邮箱、用户名、SQL、Redis key、原始错误或其他高基数字段

### Requirement: Metrics package 文档与示例

`common/runtime/observability/metrics` MUST 提供 package 文档和可执行示例，说明 provider 启停、独立 registry、重复注册、`HTTPHandler`、`GatherContext`、collector context 和 label cardinality 的稳定用法。示例 MUST 使用本地 registry 与内存 collector，MUST NOT 访问公网或真实 datastore。

#### Scenario: go doc 导航到主要示例

- **WHEN** 开发者查看 `common/runtime/observability/metrics` 的 package 文档
- **THEN** 文档 MUST 能说明 enabled/disabled provider、独立 registry、重复注册、`HTTPHandler`、`GatherContext`、collector context 和 label cardinality 约束
- **AND** go doc MUST 能导航到主要 executable examples

#### Scenario: 示例测试不依赖外部系统

- **WHEN** 执行 metrics package 的示例测试
- **THEN** 示例 MUST 只使用本地 registry、自定义 collector、`httptest` 或等价内存对象
- **AND** 示例 MUST NOT 访问公网、PostgreSQL、Redis、scheduler、workerpool 或真实 localcache datastore

### Requirement: Tracing provider 生命周期所有权

系统 MUST 为 `common/runtime/observability/tracing` provider 定义单一启动和关闭所有权。普通 constructor MUST 在返回前立即启动底层 SDK provider，并由调用方显式 `Shutdown`；Fx adapter MUST 在 constructor 阶段只返回未启动但可安全注入的 facade，并且只允许 Fx `OnStart` 创建 exporter、batch processor 和真实 SDK provider，Fx `OnStop` 或 rollback MUST 关闭同一 provider 并恢复 no-op。

#### Scenario: 普通 constructor 立即启动
- **WHEN** 调用方使用普通 constructor 创建启用 tracing 的 provider
- **THEN** constructor MUST 在调用方 context 预算内创建 OTLP exporter、batch processor 和 SDK provider
- **AND** 返回成功后调用方 MUST 通过 `Shutdown(ctx)` 关闭该 provider
- **AND** constructor 失败时 MUST 返回包含 `create OTLP tracing exporter` 或等价可定位上下文的错误，并通过标准 wrapping 保留底层 cause

#### Scenario: Fx constructor 延迟启动
- **WHEN** Fx graph 构造 tracing provider
- **THEN** constructor MUST 返回非 nil provider 和可安全注入的 dynamic tracer provider
- **AND** constructor 阶段 MUST NOT 创建 exporter、连接 OTLP endpoint、启动 batch processor 或执行可能阻塞的 tracing 初始化
- **WHEN** Fx lifecycle 执行 `OnStart(ctx)`
- **THEN** provider MUST 使用同一个实例创建真实 SDK provider，并在启动 context 预算内完成

#### Scenario: Fx rollback 关闭已启动 provider
- **WHEN** tracing `OnStart` 已成功但后续 lifecycle hook 启动失败
- **THEN** Fx rollback MUST 调用同一 provider 的 `Shutdown(ctx)`
- **AND** provider MUST 关闭 SDK provider、batch processor 和 exporter，并将后续 dynamic tracer 使用恢复为 no-op
- **AND** 系统 MUST NOT 悬挂已关闭 provider 或保留旧 exporter

#### Scenario: 禁用 tracing 不连接 OTLP
- **WHEN** tracing 配置为 disabled
- **THEN** provider MUST 保持非 nil 且可注入
- **AND** 启动路径 MUST NOT 要求 OTLP endpoint、创建 OTLP exporter 或连接网络
- **AND** span 创建 MUST 安全返回 no-op 或 never-sampled span，且不改变调用方业务语义

### Requirement: Dynamic tracer 启停安全

constructor 阶段获取的 dynamic tracer provider 和 tracer MUST 在 provider 启动前、启动后、Shutdown 后都可安全使用。启动前和 Shutdown 后 MUST 使用 no-op provider；启动后 MUST 委托当前真实 SDK provider；该切换 MUST NOT 安装 OpenTelemetry global provider，也 MUST NOT 要求 instrumentation 重新获取 tracer。

#### Scenario: 启动前 dynamic tracer 安全 no-op
- **WHEN** Redis、Gin、Ent 或其他 instrumentation 在 constructor 阶段保存 dynamic tracer 或 tracer provider
- **THEN** provider 尚未启动时 span 创建 MUST 返回安全 no-op span
- **AND** 调用方 MUST NOT 因 tracing 未启动而 panic、阻塞或连接 exporter

#### Scenario: 启动后 dynamic tracer 使用真实 provider
- **WHEN** 同一个 provider 在 lifecycle 中成功启动
- **THEN** constructor 阶段已保存的 dynamic tracer MUST 使用当前真实 SDK provider 创建 span
- **AND** 调用方 MUST NOT 重新安装 instrumentation 或重新获取 tracer provider 才能获得真实 span

#### Scenario: Shutdown 后恢复 no-op
- **WHEN** provider 已执行 `Shutdown(ctx)` 并关闭底层 SDK provider
- **THEN** 既有 dynamic tracer 后续 span 创建 MUST 回退到 no-op
- **AND** 系统 MUST NOT 使用已关闭的 SDK provider、batch processor 或 exporter

#### Scenario: 传播器保持稳定
- **WHEN** 调用方通过 provider 获取 `TextMapPropagator`
- **THEN** propagator MUST 支持 W3C trace context 与 baggage 的 inject/extract
- **AND** propagator 行为 MUST NOT 依赖 provider 是否已连接 OTLP exporter

### Requirement: Tracing lifecycle 重复调用语义

tracing provider MUST 对重复或非法 lifecycle 调用提供明确且被测试的结果。重复启动同一 provider MUST 失败并保持既有已启动 provider 不变；`Shutdown(ctx)` 对 nil provider、未启动 provider 和已关闭 provider MUST 幂等成功；非法启动输入 MUST 返回明确错误。

#### Scenario: 重复启动不得泄漏旧 provider
- **WHEN** 同一个 provider 已成功启动后再次执行启动逻辑
- **THEN** 第二次启动 MUST 返回可识别错误
- **AND** 系统 MUST 保持第一次启动的 SDK provider 仍为当前 provider
- **AND** 系统 MUST NOT 静默替换、丢失或泄漏旧 exporter、batch processor 或 SDK provider

#### Scenario: Shutdown 幂等
- **WHEN** provider 为 nil、从未启动或已经关闭
- **THEN** `Shutdown(ctx)` MUST 返回 nil
- **AND** 重复 Shutdown MUST NOT panic、阻塞或重复关闭同一 exporter

#### Scenario: 非法启动输入失败
- **WHEN** 启动逻辑收到 nil context、nil provider、缺失 resource 或启用 tracing 但缺失 exporter factory
- **THEN** 启动 MUST 返回明确错误
- **AND** provider MUST 保持未启动或保持原有已启动 provider 不变

### Requirement: 通用 metrics provider 与 feature metrics 所有权分离

系统 MUST 保持通用 metrics Provider、Prometheus registry、metrics endpoint、HTTP metrics middleware、runtime collector 和 component status collector 的跨服务边界，同时 MUST 让带业务语义的 feature metrics 由 owning feature 或服务级 adapter 拥有。permission/RBAC metrics MAY 复用通用 Provider 注册 collector，但其 recorder interface、指标定义和空实现 MUST 留在 permission 边界。

#### Scenario: 通用 provider 继续支撑 feature collector 注册

- **WHEN** metrics 启用且 permission feature 注册 Casbin reload collector
- **THEN** collector MUST 通过通用 metrics Provider 注册到同一 Prometheus registry
- **AND** `/metrics` endpoint、service/environment label 约束和 provider enabled/disabled 语义 MUST 保持不变

#### Scenario: disabled metrics 不暴露 collector

- **WHEN** metrics 禁用
- **THEN** 系统 MUST NOT 暴露 metrics endpoint 或注册 Casbin reload collector
- **AND** permission feature MUST 继续获得非 nil feature-local no-op recorder，运行时行为和依赖图 MUST 保持稳定

#### Scenario: component status collector 不承载 feature 指标

- **WHEN** user-service 通过 component status collector 暴露运行时组件状态
- **THEN** collector MUST 继续只表达业务中立的 running 和 last error 状态
- **AND** Casbin reload 计数、last success gauge、RBAC watcher 专用指标和 outbox 指标 MUST 由 permission feature metrics 拥有

#### Scenario: 架构门禁防止业务指标回流 common

- **WHEN** 运行 `make user-service-architecture-lint`
- **THEN** 检查 MUST 在 `common/runtime/observability/metrics` 出现 Casbin、permission、role、RBAC、user-service 或 `aegiscore_casbin` 业务 metrics 语义时失败
- **AND** 该门禁 MUST 不禁止 common 保留通用 Provider、通用 label、HTTP metrics 和 component status collector

