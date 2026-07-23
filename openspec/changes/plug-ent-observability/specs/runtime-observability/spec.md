## MODIFIED Requirements

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
