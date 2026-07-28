## Context

`user-service/internal/providers/ent.go` 当前在创建 Ent driver 时默认包装 SQL 观测 driver，并在创建 Ent client 后默认安装 Ent query tracing 与 Prometheus metrics。这个实现把 SQL log、tracing、metrics 绑定在同一条构造路径上，默认产出 SQL slow/error/completed 日志与 Ent query metrics，也通过旧配置 `ent.sql_debug` 控制成功 SQL debug 日志。

本次 change 是运行时观测契约和配置契约的破坏性变更，主要影响 `user-service/internal/config/`、`user-service/internal/providers/`、`docs/`、`deployments/` 和 `openspec/specs/runtime-observability/`。它不改变 HTTP API、数据库 schema、Atlas migration、OpenAPI 生成物或 RBAC 安全边界。

## Goals / Non-Goals

**Goals:**

- 将 Ent SQL log、tracing、metrics 拆成独立插件，由 `ent.plugins.*` 显式控制。
- 让 `newEntClient` 只负责基础 Ent driver 组装、插件应用和 `*ent.Client` 创建。
- 默认仅安装 Ent tracing 插件，不启用 SQL log driver wrapper，不注册 Ent query Prometheus metrics。
- 删除 `ent.sql_debug` 配置契约和相关测试、示例、文档引用。
- 保留现有 Ent query metrics 名称，避免不必要的 Prometheus/Grafana 查询迁移。
- 保留 `nonClosingEntDriver.Close()` 不关闭底层 `*sql.DB` 的语义。

**Non-Goals:**

- 不改变 Ent schema、Atlas migration、SQL 语义、事务语义或查询返回值。
- 不修改 HTTP API、OpenAPI 文档生成物或业务 feature 行为。
- 不把 user-service 私有 Ent 插件抽到 `common/runtime/observability`。
- 不新增外部依赖或新的 metrics/tracing 后端。
- 不保留 `ent.sql_debug` 的兼容读取逻辑。

## Decisions

- 决策：新增 user-service 私有插件接口 `entDriverPlugin` 与 `entClientPlugin`。
  理由：SQL log 必须包装 `dialect.Driver`，tracing/metrics 必须通过 Ent client hook/interceptor 安装，两类扩展点生命周期不同。把接口放在 `user-service/internal/providers` 可以避免 `common` 承载服务私有 Ent 组装语义。
  备选方案：使用一个统一 `EntPlugin` 同时暴露 driver/client 方法。该方案会让每个插件实现不需要的方法，职责边界较差。

- 决策：`newEntPlugins(cfg, log, metricsProvider, tracingProvider)` 统一把配置转换为 `entPluginSet`。
  理由：配置默认值、provider nil/disabled 判断和 collector 注册错误都集中在构造阶段，`newEntClient` 不再依赖全局服务配置，便于测试插件顺序和失败关闭路径。
  备选方案：在 `ProvideEntClients` 中直接拼装 plugin slices。该方案会把配置细节留在 Fx provider 主流程里，后续扩展插件时更容易膨胀。

- 决策：SQL log 插件复用并重命名旧 SQL observability driver。
  理由：旧 driver 已覆盖 Exec、Query、Tx、Commit、Rollback 的日志语义，重命名为 `entSQLLogDriver` 后可保留稳定字段和日志 message，同时通过配置显式启用。
  备选方案：重新实现一个更窄的 SQL logger。该方案增加行为漂移风险，不利于破坏性配置变更之外的运维稳定性。

- 决策：tracing 插件和 metrics 插件分别安装独立 Ent interceptor/hook。
  理由：tracing 和 metrics 的启停条件不同，metrics collector 注册可能失败且必须阻止 client 创建；二者拆分后互不强绑定，更符合插件化目标。
  备选方案：继续使用复合 `installEntObservability`。该方案不能表达“tracing 开启但 metrics 关闭”与“metrics 开启但 tracing provider 为空”的独立语义。

- 决策：默认配置为 `sql_log.enabled=false`、`tracing.enabled=true`、`metrics.enabled=false`。
  理由：默认保留低开销 tracing 行为，移除高噪声 SQL log 和默认 metrics collector 注册，避免没有显式运维意图时产生额外日志和 Prometheus series。
  备选方案：默认全部关闭。该方案会退化现有依赖 tracing 的运行时诊断能力，不符合 user-service 当前 tracing contract。

## Risks / Trade-offs

- [风险] 默认不再注册 Ent query metrics，现有 Grafana 面板或告警可能无数据。→ 缓解：更新文档和部署资产说明，只有 `ent.plugins.metrics.enabled=true` 且 metrics provider 启用时相关指标才有数据。
- [风险] 默认不再输出 Ent SQL slow/error 日志，依赖旧日志排障的环境会少一类信号。→ 缓解：保留旧日志 message 和字段，迁移为显式 `ent.plugins.sql_log.enabled=true`。
- [风险] metrics collector 注册冲突会在 Ent client 构造阶段失败。→ 缓解：保持错误向上返回并覆盖测试，避免重复注册后服务以不完整观测状态启动。
- [风险] client plugin 安装失败后已创建的 Ent client 未关闭会泄漏 driver 资源。→ 缓解：`newEntClient` 在任一 client plugin 返回错误时调用 `client.Close()`，并测试失败路径。
- [风险] 配置迁移遗漏导致启动配置不符合新契约。→ 缓解：全仓搜索 `ent.sql_debug` 和 `sql_debug`，同步修改 docs、deployments、测试 fixture 与配置测试。

## Migration Plan

1. 修改配置结构，删除 `EntConfig.SQLDebug`，新增 `EntPluginsConfig`、`EntSQLLogPluginConfig`、`EntTracingPluginConfig`、`EntMetricsPluginConfig`，保持 `slow_threshold` 使用仓库现有 duration decode 方式。
2. 新增 `entPluginSet`、driver/client plugin 接口和 `newEntPlugins` 构造函数。
3. 将旧 SQL observability driver 重命名并迁移为 SQL log driver plugin。
4. 将 Ent tracing 和 metrics 安装逻辑拆为独立插件，删除旧 `installEntObservability` 和 `entSQLDebugEnabled`。
5. 修改 `ProvideEntClients`、`newEntClient`、`newEntDriver` 主流程。
6. 同步测试、文档、部署配置和观测资产说明。
7. 验证 `go test ./user-service/internal/providers ./user-service/internal/config`、`make user-service-architecture-lint`，按影响范围运行 `make user-service-test`。

回滚方式：在代码层回滚本 change，恢复 `ent.sql_debug` 和旧默认安装行为；在部署层可通过启用 `ent.plugins.sql_log.enabled=true` 或 `ent.plugins.metrics.enabled=true` 临时恢复部分观测信号，但旧配置字段不提供兼容回滚。

## Open Questions

无。
