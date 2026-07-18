## Purpose

定义 user-service 和共享 runtime 的可观测性能力，覆盖健康检查、OpenAPI、metrics、tracing、日志、运行时故障处理和部署观测资产。
## Requirements
### Requirement: 健康检查与运行时端点

系统 MUST 在业务 API 之外提供 `/livez`、`/readyz`、`/startupz`、配置化 metrics endpoint、OpenAPI 文档端点和可选 pprof 诊断监听，并保持这些端点的访问边界和启停语义明确。健康检查 MUST 只通过稳定 public contract 读取跨 feature 运行状态，MUST NOT 直接依赖 feature infrastructure concrete implementation。

#### Scenario: 存活、就绪与启动检查

- **WHEN** 调用 `/livez`
- **THEN** endpoint MUST 只证明进程可响应，并 MAY 在外部依赖异常时继续成功
- **WHEN** PostgreSQL、Redis、Casbin policy 或 policy watcher 等就绪依赖不可用
- **THEN** `/readyz` 或 `/startupz` MUST 失败并返回可定位且不含 secret、DSN、SQL、token、Cookie、stacktrace 的信息

#### Scenario: 运行时路由不经过业务授权

- **WHEN** user-service 注册健康检查、OpenAPI 或 metrics 路由
- **THEN** 路由 MUST 位于 `/api/v1` 之外，MUST NOT 经过 RBAC 业务授权
- **AND** metrics 配置无效时路由注册 MUST 返回错误，而不是静默使用错误配置

#### Scenario: HTTP 服务禁用

- **WHEN** `server.http.enabled=false`
- **THEN** user-service MUST 不启动 HTTP 监听
- **AND** 依赖 HTTP 的健康检查、OpenAPI 和 metrics 路由 MUST 不对外暴露

#### Scenario: pprof 受控暴露

- **WHEN** pprof 未显式启用
- **THEN** 系统 MUST 不启动 pprof listener
- **WHEN** pprof 显式启用
- **THEN** 系统 MUST 使用独立诊断 listener，并默认限制在 loopback 或受控网络边界

#### Scenario: 健康检查依赖 public contract

- **WHEN** service-level provider 构造 Casbin policy 或 policy watcher 健康检查
- **THEN** provider MUST 依赖 permission feature 暴露的只读 health/status interface
- **AND** provider MUST NOT import permission infrastructure casbin、redis watcher 或其他 concrete implementation 包

### Requirement: user-service HTTP route 总装边界

系统 MUST 由 user-service composition root 统一维护 HTTP route 的访问层级，并通过明确的 route registrar contract 接入 feature 路由。route registrar MUST 按 public、authenticated 和 authorized 层级注册，MUST NOT 依赖 Fx value group 的 slice 顺序表达安全或冲突语义。

#### Scenario: 分层注册业务路由

- **WHEN** user-service 注册 `/api/v1` 路由
- **THEN** public auth route MUST 不经过普通 access token middleware
- **AND** authenticated auth route MUST 经过 token version validator 认证 middleware
- **AND** permission、role 和 user 业务 route MUST 先经过认证 middleware，再经过 RBAC authorizer middleware

#### Scenario: 新增 feature route

- **WHEN** 新 feature 需要挂载 `/api/v1` 业务路由
- **THEN** feature MUST 通过对应访问层级的 route registrar contract 接入 composition root
- **AND** 新 feature MUST NOT 要求在 `RegisterRouteParams` 或 `router.RouteParams` 中新增 feature controller 字段

#### Scenario: 禁止依赖 value group 顺序

- **WHEN** route registrar 通过 Fx value group 注入
- **THEN** 注册逻辑 MUST NOT 假设 group slice 顺序稳定
- **AND** 如果存在 path 冲突、顺序或 middleware 层级要求，composition root MUST 使用显式编排或稳定排序规则表达该要求

#### Scenario: route graph 行为保持

- **WHEN** route registrar 化完成后运行 route graph 测试或 route diff 诊断
- **THEN** 已有健康检查、OpenAPI、metrics、auth、permission、role 和 user route 的 path、method、访问层级和 route template MUST 保持不变
- **AND** 本变更 MUST NOT 改变 OpenAPI 文档语义或业务 controller 行为

### Requirement: runtime dependency 观测使用稳定资源名

系统 MUST 使用稳定低基数资源名标识 runtime dependency 指标、健康检查和告警。user-service 主 PostgreSQL runtime dependency 的资源名 MUST 为 `primary_db`，Redis 缓存资源名保持 `cache_redis`。

#### Scenario: PostgreSQL runtime dependency 暴露观测标签

- **WHEN** user-service 注册 PostgreSQL 连接池指标、健康检查或告警查询
- **THEN** PostgreSQL 资源名 MUST 使用 `primary_db`
- **AND** 指标 label、健康检查名称、dashboard 查询和 alert 表达式 MUST 保持一致

### Requirement: OpenAPI 文档契约

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

#### Scenario: 登录响应分支

- **WHEN** 生成登录接口文档
- **THEN** 普通登录 MUST 表达成功 envelope 和 access/refresh token，强制改密登录 MUST 表达 `CodePasswordChangeRequired` 及受限 access token
- **AND** 两个分支 MUST 复用 `TokenResponse`，MUST NOT 引入 `status`、`authenticated`、`password_change_required` 枚举或独立 `LoginResponse`
- **AND** KDF busy 的 `503 Service Unavailable` MUST 继续被声明

### Requirement: Metrics 平台与低基数契约

系统 MUST 提供 Prometheus metrics 基础能力，并以非 nil provider 显式表达启用或禁用状态。HTTP、runtime、scheduler、workerpool、SQL、Redis 和 feature metrics MUST 保持稳定、低基数且不泄露敏感数据。

#### Scenario: metrics 启停和标签

- **WHEN** metrics 暴露被启用
- **THEN** 系统 MUST 注册配置化 metrics endpoint 并导出已注册 collector
- **WHEN** metrics 被禁用
- **THEN** 系统 MUST 不暴露 endpoint 或 collector，但 MUST 向正式依赖图提供非 nil no-op provider
- **AND** label MUST NOT 包含用户、角色、权限、会话或 token ID、trace/span ID、raw path、IP、邮箱、用户名、SQL、Redis key 或原始错误

#### Scenario: HTTP in-flight gauge 正确归零

- **WHEN** metrics middleware 跳过 runtime endpoint 或其他配置化请求
- **THEN** 请求总数和耗时 MAY 不记录该请求
- **AND** in-flight gauge MUST 在请求结束后递减到 `0`，MUST NOT 因删除共享 label value 破坏并发计数

#### Scenario: Fx provider 组合

- **WHEN** Fx graph 使用有效共享 runtime config 构造 metrics 或 tracing provider
- **THEN** provider MUST 投影 service name、environment 和对应配置，传播构造错误，并为 tracing 注册 `OnStop: provider.Shutdown`
- **WHEN** provider 收到 nil config
- **THEN** 构造 MUST 返回明确错误，MUST NOT 静默使用默认配置

#### Scenario: 正式依赖图完整

- **WHEN** `providers.Module` 构建 Ent、auth 或 permission 的正式图
- **THEN** metrics/tracing 以及 feature-local `Metrics` 输入 MUST 是非 optional 的明确依赖
- **AND** 启用时 MUST 注入真实 recorder，禁用时 MUST 注入 no-op 实现，缺失依赖 MUST 导致构图失败
- **AND** 直接构造测试 MAY 使用 nil 防御，但其 MUST NOT 成为正式降级机制

#### Scenario: feature metrics no-op 归属

- **WHEN** feature-local `Metrics` interface 需要空实现
- **THEN** 系统 MUST 通过统一生成入口维护匹配接口的 no-op 实现
- **AND** 业务指标方法 MUST 留在所属 feature，`common/runtime/observability/metrics` MUST NOT 承载 user-service 业务语义

#### Scenario: 只读观测集合

- **WHEN** 代码读取低基数 label allowlist、HTTP label names 或 scheduler duration buckets
- **THEN** 调用方 MUST 获得不可共享写入的值，顺序和数值 MUST 保持稳定
- **AND** 包内误写 MUST NOT 改变后续指标契约

### Requirement: 本地缓存运行时指标

系统 MUST 为 `common/runtime/localcache` 导出低基数 `aegiscore_localcache_*` 指标，覆盖请求、回源、singleflight、写入、驱逐和容量，并由 dashboard、alert 和真实 metrics load 校验消费当前稳定契约。

#### Scenario: 请求和回源指标

- **WHEN** cache 命中、未命中、执行 loader 或 loader 失败
- **THEN** 系统 MUST 记录 hit、miss、load 和 load error counter
- **AND** 标签 MUST 仅使用固定 cache 名与固定枚举，MUST NOT 包含 raw key、身份标识或原始错误

#### Scenario: 防击穿和容量指标

- **WHEN** singleflight 合并并发 miss、内部 double-check 命中、Ristretto 丢弃写入、拒绝准入或驱逐条目
- **THEN** 系统 MUST 分别记录 shared result、double-check、set dropped、admission rejected、evicted 和 capacity 指标
- **AND** shared result 与 double-check MUST NOT 计入业务 hit ratio

#### Scenario: 观测资产消费当前指标

- **WHEN** Grafana、Prometheus alert 或 metrics load 脚本消费本地缓存指标
- **THEN** 其 MUST 使用当前 `aegiscore_localcache_*` metric family 和 `cache`、`result`、`event` 等固定标签
- **AND** 旧名称、旧标签和兼容 PromQL MUST NOT 被保留

#### Scenario: metrics 禁用时保留本地统计

- **WHEN** metrics provider 被禁用
- **THEN** localcache collector MUST 不注册
- **AND** localcache MUST 继续维护可由 `Stats()` 读取的本地快照

### Requirement: HTTP 请求关联与日志安全

系统 MUST 为每个 HTTP 请求建立 request ID，并通过共享 logger context 将其与 access log、应用日志和有效 tracing context 关联。日志 MUST 结构化输出到 stdout/stderr，保持分级且不得记录敏感信息。

#### Scenario: 透传或生成 request ID

- **WHEN** 入站 `X-Request-ID` 合法
- **THEN** 系统 MUST 在响应头和请求日志中透传相同值
- **WHEN** header 缺失、空白、超长或含控制字符
- **THEN** 系统 MUST 生成新值并用于响应头、access log 和应用日志

#### Scenario: request ID API 归属

- **WHEN** middleware 写入最终 request ID
- **THEN** MUST 使用 `common/runtime/logger` 的 `WithRequestID`，并可由 `RequestIDFromContext` 读取
- **AND** `common/http/middleware` MUST NOT 保留同名 context API、兼容别名或 deprecated wrapper

#### Scenario: request ID 与 trace 并存

- **WHEN** 请求具有有效 W3C trace context
- **THEN** 日志 MUST 同时包含独立的 `request_id`、`trace_id` 和 `span_id`
- **WHEN** span context 无效
- **THEN** 日志 MUST 省略 trace 字段但保留有效 request ID
- **AND** metrics label MUST NOT 包含这些关联 ID

#### Scenario: 日志和 panic 可观测

- **WHEN** 请求完成、发生 panic 或 span 对应操作失败
- **THEN** access log MUST 记录稳定字段，recovery MUST 记录错误并返回统一响应，span MUST 标记错误
- **AND** 日志 MUST NOT 包含密码、token、Cookie、Authorization、DSN、SQL 参数或完整 Redis key

#### Scenario: logger 生命周期与隔离

- **WHEN** 正式 App 启停或多个 App 并行运行
- **THEN** App MUST 使用显式注入的 logger，在 Stop 时同步自身 logger，且 MUST NOT 安装或依赖进程级默认 logger
- **AND** logger 默认值相关测试 MUST 隔离并恢复进程状态

### Requirement: Tracing 与依赖观测

系统 MUST 通过最小 OTLP 配置提供 OpenTelemetry tracing，并为 Redis 命令、Ent 查询和 HTTP 请求传播上下文；禁用 tracing 时 MUST 保持非 nil no-op 语义。启用 tracing 时，Fx provider MUST 在 lifecycle `OnStart(ctx)` 中初始化 exporter 和 SDK provider，并在 `OnStop(ctx)` 中关闭，MUST NOT 在 `fx.New` constructor 阶段连接 exporter。

#### Scenario: tracing 启停

- **WHEN** tracing 关闭
- **THEN** provider MUST 使用 no-op 或 `NeverSample` 语义且不连接 exporter
- **WHEN** tracing 开启且 Fx app 执行 `OnStart(ctx)`
- **THEN** provider MUST 使用服务名、环境和 OTLP endpoint 初始化 exporter 与 SDK provider
- **AND** exporter 初始化 MUST 使用 lifecycle 启动 context，受 Fx 启动预算、取消和超时控制
- **AND** lifecycle 停止时 provider MUST 使用停止 context 关闭 SDK provider 和 exporter 资源

#### Scenario: tracing exporter 构造失败

- **WHEN** tracing 开启且 OTLP exporter 构造失败
- **THEN** `OnStart(ctx)` MUST 返回包含 `create OTLP tracing exporter` 语义的错误
- **AND** 返回错误 MUST 通过标准错误 wrapping 保留底层 gRPC、TLS、endpoint 或 context cause
- **AND** 系统 MUST NOT 将底层 cause 替换为无 cause 的新错误

#### Scenario: constructor 阶段不连接 exporter

- **WHEN** Fx graph 在 `fx.New` constructor 阶段构造 tracing provider
- **THEN** provider 对象 MUST 可注入给依赖方
- **AND** provider MUST NOT 连接 OTLP exporter 或执行可能阻塞的 exporter 初始化
- **AND** 依赖方 MUST NOT 要求 `TracerProvider()` 在 `OnStart` 前已经连接真实 exporter

#### Scenario: Redis 命令 span

- **WHEN** user-service 执行 Redis 命令
- **THEN** 系统 MUST 创建低风险属性的 span 并传播服务 tracing provider
- **AND** span MUST NOT 记录完整 key、参数、token、密码或连接 secret

#### Scenario: Redis metrics 探测取消

- **WHEN** metrics HTTP scrape context 被取消
- **THEN** Redis PING MUST 尽快终止
- **WHEN** collector 经标准 `Collect` 直接调用
- **THEN** MUST 使用 background context 与 collector timeout，不得声称感知 HTTP 取消
- **AND** 最小探测间隔、快照复用及 `aegiscore_redis_*` 指标契约 MUST 保持不变

#### Scenario: Ent 查询观测

- **WHEN** Ent 执行查询
- **THEN** 系统 MUST 产生 span，并记录低基数 latency 与 error metrics
- **AND** 观测 MUST NOT 修改 SQL、事务、schema、查询返回值或错误语义

### Requirement: 业务安全指标与部署观测资产

系统 MUST 维护 Prometheus alerts、Grafana dashboards、Compose 观测配置、生成脚本和 runbook，使 RBAC、认证安全及 runtime 关键指标具有可行动的观测视图且不会引入高基数。

#### Scenario: RBAC Enforce 延迟

- **WHEN** dashboard 展示 RBAC Enforce 性能
- **THEN** MUST 使用低基数 histogram 展示 P95 和 P99，并同步到源码与 Compose provisioning dashboard

#### Scenario: 强制改密安全信号

- **WHEN** 一次性会话消费、重复消费拒绝、撤销投影或补偿失败
- **THEN** 系统 MUST 记录对应低基数指标
- **AND** alert 与 metrics load 校验 MUST 覆盖可导致安全撤销失效的信号并指向稳定 runbook

#### Scenario: 观测资产生成和 drift

- **WHEN** dashboard source 或生成逻辑变化
- **THEN** 生成脚本 MUST 更新 provisioning JSON
- **AND** `make compose-dashboard-check` 或等价校验 MUST 在生成物 drift 时失败

### Requirement: 监听故障与优雅关闭

系统 MUST 将 HTTP 或 pprof listener 的非预期退出转换为 Fx shutdown signal，并在统一的 `runtime.lifecycle.stop_timeout` 总预算内按逆序 lifecycle hook 完成优雅关闭。pprof listener 在 graceful shutdown 失败时 MUST 执行 best-effort 强制关闭，避免 listener 或 `Serve` goroutine 滞留至进程退出。

#### Scenario: listener 非预期退出

- **WHEN** HTTP 或 pprof `Serve` 在未进入正常关闭阶段时返回错误
- **THEN** 系统 MUST 记录可诊断错误并触发非零内部 shutdown signal
- **WHEN** 正常关闭导致 `http.ErrServerClosed`
- **THEN** 系统 MUST NOT 将其视为内部故障

#### Scenario: 外部与内部退出共用预算

- **WHEN** 外部终止信号或内部故障触发关闭
- **THEN** 系统 MUST 使用同一未被取消的上游 context value 和 `runtime.lifecycle.stop_timeout` 总预算执行 `App.Stop`
- **AND** 局部 HTTP、gRPC、tracing 或 logger timeout MUST NOT 替代总预算

#### Scenario: 前序 hook 消耗时间

- **WHEN** 前序 `OnStop` hook 已消耗部分总预算
- **THEN** 后续 hook MUST 只使用剩余时间
- **AND** 总关闭耗时 MUST NOT 因每个组件重新创建完整预算而无界增长

#### Scenario: lifecycle timeout 同源

- **WHEN** App 和 CLI 构建启动或停止 context
- **THEN** 两者 MUST 使用同一已加载并校验的 lifecycle 配置
- **AND** `fx.New` 构造期 MUST NOT 被误算入 `StartTimeout`，也 MUST NOT 为满足 timeout 而隐式迁移现有资源构造语义

#### Scenario: 快速正常关闭

- **WHEN** 所有 hook 在预算内完成
- **THEN** App MUST 立即完成关闭，不得等待完整 timeout

#### Scenario: pprof graceful shutdown 失败后强制关闭

- **WHEN** pprof 已启用且 `OnStop` 调用 `server.Shutdown(ctx)` 返回错误
- **THEN** 系统 MUST 对同一个 pprof server 执行 best-effort `server.Close()`
- **AND** 返回错误 MUST 保留 `Shutdown` 失败信息
- **AND** 当 `Close` 也失败时，返回错误 MUST 同时包含强制关闭失败信息
- **AND** `Serve` goroutine MUST 因 listener 关闭退出，且正常关闭产生的 `http.ErrServerClosed` MUST NOT 触发非零内部 shutdown signal

#### Scenario: pprof 停止幂等性

- **WHEN** pprof server 已经被关闭后再次进入停止路径
- **THEN** 系统 MUST NOT 因重复 `Close` 导致 panic 或阻塞
- **AND** 重复停止产生的关闭错误 MUST 作为诊断返回或被原有 `http.Server` 语义吸收

### Requirement: Fx tracing 资源启动失败安全

系统 MUST 在 Fx 装配 tracing provider 时避免 constructor 阶段创建带后台副作用的 tracing batch processor 或 exporter；真实 tracing 运行资源 MUST 在 `OnStart` 中创建，并 MUST 在 `OnStop` 中关闭。禁用 tracing 时 MUST 保持非 nil no-op 语义且不得连接 exporter。

#### Scenario: 后续启动失败不泄漏 tracing 资源
- **WHEN** tracing 已启用且 Fx App 在 tracing `OnStart` 成功后因后续 hook 失败而启动失败
- **THEN** App MUST 关闭 tracing `OnStart` 创建的 provider、batch processor 和 exporter
- **AND** 关闭错误 MUST 被保留或记录为可诊断信息，不得静默吞掉

#### Scenario: constructor 阶段无后台副作用
- **WHEN** Fx graph 构造 tracing provider 依赖但 App 尚未执行 `Start`
- **THEN** tracing constructor MUST NOT 启动 batch processor、建立 OTLP exporter 连接或注册需要 stop hook 才能清理的后台资源
- **AND** 无效静态配置仍 MUST 在构造或启动阶段返回明确错误

### Requirement: Ent 观测 wrapper 生命周期安全

系统 MUST 保持 Ent 查询 tracing 和 metrics 观测语义不变，并确保 Ent wrapper 或观测资源在启动失败路径中不会依赖未执行的 stop hook 才完成清理。

#### Scenario: Ent wrapper 部分构造失败回滚
- **WHEN** user-service 构造 Ent client wrapper 或其观测依赖时后续步骤失败
- **THEN** 已创建且需要关闭的部分资源 MUST 立即关闭
- **AND** 返回错误 MUST 保留原始构造失败和清理失败信息

### Requirement: Fx 初始化事件使用统一结构化日志

系统 MUST 将 user-service 正式 Fx App 的 Fx event logger 接入当前 App 注入的结构化 `*zap.Logger`，并使用与业务日志一致的 encoder、字段和输出目标记录 Fx 构图、Invoke、constructor、decorator、stub、rollback、lifecycle 和 module trace 事件。

#### Scenario: 正式 App 构造时启用 Fx event logger

- **WHEN** user-service 通过 `AppOptions` 或 `NewApp` 构建正式 Fx App
- **THEN** Fx event logger MUST 由已注入的 `*zap.Logger` 构造
- **AND** Fx 自身事件 MUST 使用命名 logger 输出到统一结构化日志链路

#### Scenario: Fx event 日志级别

- **WHEN** Fx 记录常规构图、执行前后、module trace 或 lifecycle 事件
- **THEN** 系统 MUST 使用 debug 级别记录这些事件
- **WHEN** Fx 记录构造、Invoke、rollback 或 lifecycle 失败事件
- **THEN** 系统 MUST 使用 error 级别记录失败事件

#### Scenario: Fx event logger 保持快速非阻塞

- **WHEN** Fx 调用 event logger 记录初始化或 lifecycle 事件
- **THEN** event logger MUST 只执行本地 logger adapter 逻辑
- **AND** event logger MUST NOT 在 `LogEvent` 路径执行网络 I/O、远程导出、阻塞式重试或业务副作用

#### Scenario: logger 生命周期语义保持不变

- **WHEN** Fx App 停止并释放共享 logger
- **THEN** 系统 MUST 继续由 logger provider 的 Stop hook 同步当前 App logger
- **AND** Fx event logger MUST NOT 替换进程级默认 logger 或引入额外同步生命周期

### Requirement: Fx constructor 阶段 tracing provider 可用

系统 MUST 在依赖 tracing 的 Redis、Gin、Ent 等 user-service provider constructor 执行前，向 Fx 依赖图提供非 nil 且底层 `TracerProvider()` 可用的 tracing provider，并在 Fx 停止或启动 rollback 时关闭该 provider。

#### Scenario: constructor 阶段消费 tracing provider

- **WHEN** user-service Fx graph 构造 Redis、Gin 或 Ent provider
- **THEN** tracing provider MUST 已经返回非 nil `TracerProvider()`
- **AND** 依赖 tracing 的 constructor MUST NOT 因 tracing provider 尚未进入 `OnStart` 而失败

#### Scenario: 后续启动 hook 失败时释放 tracing provider

- **WHEN** tracing provider 已经构造成功且后续 Fx lifecycle hook 启动失败
- **THEN** Fx rollback MUST 调用 tracing provider shutdown
- **AND** shutdown 后 tracing provider 的底层 `TracerProvider()` MUST 被清空

### Requirement: Fx DI 初始化边界保护
user-service composition root MUST 启用 Fx constructor、decorator 和 Invoke 范围内的 panic recovery，并 MUST 将其定位为 DI 初始化边界保护。可预期的资源、配置和依赖错误 MUST 优先通过 constructor 返回 `error` 暴露，MUST NOT 依赖 panic recovery 表达正常失败路径。

#### Scenario: constructor panic 转换为 Fx 错误
- **WHEN** Fx 在 user-service composition root 中执行 constructor、decorator 或 Invoke 时发生未预期 panic
- **THEN** App 构造或启动 MUST 通过 Fx error 暴露 panic 信息
- **AND** 进程 MUST NOT 因该 DI 初始化 panic 直接崩溃

#### Scenario: recovery 范围受限
- **WHEN** HTTP handler、worker task、后台 goroutine 或 lifecycle hook 运行期发生 panic
- **THEN** `fx.RecoverFromPanics()` MUST NOT 被视为这些运行期边界的恢复策略
- **AND** 对应边界 MUST 使用其自身已有或显式设计的 panic 处理机制

### Requirement: tracing provider constructor 可用性
Fx tracing provider MUST 在 constructor 阶段提供非 nil tracer provider，使 Redis、Gin、Ent 和其他依赖 tracing 的 constructor 可以使用同一 service runtime config 初始化 instrumentation。tracing provider MUST 继续在 Fx lifecycle stop 阶段关闭，并保持禁用 tracing 时的 no-op 或 `NeverSample` 语义。

#### Scenario: constructor 阶段使用 tracing provider
- **WHEN** user-service Fx graph 构造依赖 tracing 的 Redis、Gin 或 Ent provider
- **THEN** tracing provider 的 `TracerProvider()` MUST 返回非 nil 值
- **AND** 这些 provider MUST 使用服务级 tracing provider，不得静默回退到全局 provider

#### Scenario: tracing 构造失败
- **WHEN** tracing 配置缺失服务名、环境、非法采样率，或启用 tracing 但缺少 OTLP endpoint
- **THEN** Fx graph MUST 返回明确构造错误
- **AND** 系统 MUST NOT 延迟到 Redis、Gin、Ent 或 HTTP server 初始化时才暴露该配置错误

#### Scenario: Redis instrumentation 失败
- **WHEN** Redis tracing instrumentation 返回错误
- **THEN** Redis client constructor MUST 返回包含 `instrument redis tracing` 的错误并关闭已创建 client
- **AND** user-service cache Redis provider MUST 包装资源名并通过 Fx error path 传播该错误，MUST NOT panic

### Requirement: pprof 与 Gin mode 使用显式运行时配置

系统 MUST 通过已解析 runtime config 控制独立 pprof 诊断 listener 和进程级 Gin mode。pprof 的启用状态和监听地址 MUST 来自 `observability.pprof`，Gin mode MUST 来自 `runtime.gin.mode`；user-service 的 Fx constructor MUST NOT 直接读取裸环境变量或在 Gin engine constructor 中隐式修改 Gin 全局 mode。

#### Scenario: pprof 未启用

- **WHEN** `observability.pprof.enabled=false`
- **THEN** user-service MUST 构造可测试的 pprof handler
- **AND** user-service MUST NOT 注册或启动 pprof listener lifecycle hook

#### Scenario: pprof 启用

- **WHEN** `observability.pprof.enabled=true` 且 `observability.pprof.addr` 合法
- **THEN** user-service MUST 使用该地址启动独立 pprof listener
- **AND** pprof listener MUST 与业务 Gin router 分离

#### Scenario: pprof 配置来源

- **WHEN** 构造 `NewPprofServer`
- **THEN** constructor MUST 只消费已解析 `*config.Config`
- **AND** constructor MUST NOT 调用 `os.LookupEnv`、`os.Getenv` 或读取 `PPROF_ENABLED`、`PPROF_ADDR`

#### Scenario: Gin mode 显式初始化

- **WHEN** user-service 正式 Fx graph 需要构造 Gin engine
- **THEN** graph MUST 先基于 `runtime.gin.mode` 显式设置 Gin mode
- **AND** `NewGinEngine` MUST NOT 调用 `gin.SetMode`
- **AND** Fx 依赖 MUST 能表达 Gin mode 初始化先于 Gin engine 构造完成

### Requirement: 运行时关闭测试具备诊断与硬超时

runtime observability 相关的 HTTP drain、pprof shutdown、Fx lifecycle shared deadline 和 tracing exporter shutdown 测试 MUST 保留 Fx event 或组件日志诊断信息，并对阻塞关闭路径提供测试级硬超时保护。

#### Scenario: HTTP drain timeout 测试不无限等待

- **WHEN** 测试 HTTP server shutdown、active handler drain 或 drain tracker timeout
- **THEN** 测试 MUST 使用明确的 context deadline 和测试级等待上限
- **AND** handler、drain tracker 或 shutdown hook 忽略 context 时测试 MUST 快速失败而不是等待全局测试 timeout

#### Scenario: pprof shutdown timeout 测试不无限等待

- **WHEN** 测试 pprof server 停止、重复停止或强制关闭行为
- **THEN** 测试 MUST 对 OnStop 调用或后台请求等待设置测试级 guard
- **AND** 失败输出 MUST 保留可定位 pprof shutdown 或 listener 错误的日志信息

#### Scenario: tracing exporter start 或 shutdown 阻塞

- **WHEN** 测试 tracing exporter 创建、启动 context 或 shutdown 行为
- **THEN** 测试 MUST 使用带 timeout 的 start/stop context 或 `fxtest.EnforceTimeout(true)` 保护可阻塞 hook
- **AND** exporter 忽略 context 时测试 MUST 在测试级 guard 内失败

#### Scenario: Fx lifecycle shared deadline 可诊断

- **WHEN** 测试 Fx lifecycle stop 顺序或剩余 deadline 传播
- **THEN** 测试 MUST 使用测试 logger 或可观察断言保留 hook 执行诊断信息
- **AND** 可能阻塞的 hook 测试 MUST 启用硬超时保护
