## Purpose

定义 user-service 和共享 runtime 的可观测性能力，覆盖健康检查、OpenAPI、metrics、tracing、日志、运行时故障处理和部署观测资产。

## Requirements

### Requirement: 健康检查与运行时端点

系统 MUST 在业务 API 之外提供 `/livez`、`/readyz`、`/startupz`、配置化 metrics endpoint、OpenAPI 文档端点和可选 pprof 诊断监听，并保持这些端点的访问边界和启停语义明确。

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

系统 MUST 通过最小 OTLP 配置提供 OpenTelemetry tracing，并为 Redis 命令、Ent 查询和 HTTP 请求传播上下文；禁用 tracing 时 MUST 保持非 nil no-op 语义。

#### Scenario: tracing 启停

- **WHEN** tracing 关闭
- **THEN** provider MUST 使用 no-op 或 `NeverSample` 语义且不连接 exporter
- **WHEN** tracing 开启
- **THEN** provider MUST 使用服务名、环境和 OTLP endpoint 初始化并在 lifecycle 停止时关闭

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

系统 MUST 将 HTTP 或 pprof listener 的非预期退出转换为 Fx shutdown signal，并在统一的 `runtime.lifecycle.stop_timeout` 总预算内按逆序 lifecycle hook 完成优雅关闭。

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
