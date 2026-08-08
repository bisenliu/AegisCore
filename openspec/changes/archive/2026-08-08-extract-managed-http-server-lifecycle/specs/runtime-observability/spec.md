## MODIFIED Requirements

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
