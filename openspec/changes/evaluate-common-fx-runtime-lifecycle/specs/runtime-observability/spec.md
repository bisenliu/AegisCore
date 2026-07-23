## MODIFIED Requirements

### Requirement: Metrics 平台、依赖资源名与低基数
系统 MUST 提供 Prometheus metrics 基础能力，并以非 nil provider 显式表达启用或禁用状态。HTTP、runtime、scheduler、workerpool、SQL、Redis 和 feature metrics MUST 保持稳定、低基数且不泄露敏感数据。user-service 主 PostgreSQL runtime dependency 的资源名 MUST 为 `primary_db`，Redis 缓存资源名保持 `cache_redis`。user-service 默认 metrics `service` label、tracing `service.name`、日志 `service` 字段、健康响应 service 字段、dashboard 变量和 alert 表达式 MUST 统一使用 `aegiscore-user-service`；旧 `aegiscore-user-services` label 和兼容 PromQL MUST NOT 保留。metrics 的 Fx provider 公开入口 MUST 表达 metrics 能力语义，并由服务 composition root 显式装配。

#### Scenario: metrics 启停和依赖图完整
- **WHEN** metrics 暴露被启用
- **THEN** 系统 MUST 注册配置化 metrics endpoint 并导出已注册 collector
- **WHEN** metrics 被禁用
- **THEN** 系统 MUST 不暴露 endpoint 或 collector，但 MUST 向正式依赖图提供非 nil no-op provider
- **AND** metrics/tracing 以及 feature-local `Metrics` 输入 MUST 是非 optional 的明确依赖，缺失依赖 MUST 导致构图失败
- **AND** metrics Fx provider 的公开名称 MUST 能从 user-service composition root 的调用点识别其提供 metrics provider，不得以缺少能力语义的通用 `NewFxProvider` 作为主要入口

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

### Requirement: Tracing 与依赖观测生命周期
系统 MUST 通过最小 OTLP 配置提供 OpenTelemetry tracing，并为 Redis 命令、Ent 查询和 HTTP 请求传播上下文。constructor MUST 返回稳定、非 nil、可被 instrumentation 安全引用的 tracing facade；禁用或尚未启动时其底层 MUST 为 no-op。启用 tracing 后，`OnStart` MUST 创建 exporter 与 SDK provider 并安装到底层 facade；`OnStop` 或启动 rollback MUST 关闭真实资源并恢复 no-op。tracing 的 Fx provider 公开入口 MUST 表达 tracing 能力语义，并由服务 composition root 显式装配。

#### Scenario: tracing 启停和 constructor 阶段语义
- **WHEN** tracing 关闭或 Fx graph 在 `fx.New` constructor 阶段构造 tracing provider
- **THEN** provider MUST 可注入给 Redis、Gin、Ent 等依赖方，并提供非 nil no-op tracer provider
- **AND** constructor 阶段 MUST NOT 连接 OTLP exporter、启动 batch processor 或执行可能阻塞的 exporter 初始化
- **AND** tracing Fx provider 的公开名称 MUST 能从 user-service composition root 的调用点识别其提供 tracing provider，不得以缺少能力语义的通用 `NewFxProvider` 作为主要入口
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

### Requirement: 运行时故障、初始化保护与优雅关闭
系统 MUST 将 HTTP 或 pprof listener 的非预期退出转换为 Fx shutdown signal，并在统一的 `runtime.lifecycle.stop_timeout` 总预算内按逆序 lifecycle hook 完成优雅关闭。可预期的资源、配置和依赖错误 MUST 优先通过 constructor 返回 `error` 暴露，MUST NOT 依赖 panic recovery 表达正常失败路径。user-service composition root MUST 显式表达 process runtime 初始化、observability provider、feature lifecycle module 和 runtime server 注册之间的绑定关系。

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

#### Scenario: 服务 composition root 显式绑定 runtime lifecycle
- **WHEN** user-service 构建正式 Fx App 或装配测试 App
- **THEN** composition root MUST 显式绑定 process runtime 初始化、metrics provider、tracing provider、服务资源 provider、feature lifecycle module 和 runtime server 注册
- **AND** process runtime 初始化 MUST 在 HTTP、pprof 或其他 runtime server 启动前执行
- **AND** common/runtime/observability provider MUST 保持业务中立，不得导入 user-service feature、router、bootstrap 或服务私有配置包
