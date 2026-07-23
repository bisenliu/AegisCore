## 1. 配置契约

- [x] 1.1 在 `user-service/internal/config/config.go` 删除 `EntConfig.SQLDebug`，新增 `EntPluginsConfig`、`EntSQLLogPluginConfig`、`EntTracingPluginConfig` 和 `EntMetricsPluginConfig`。
- [x] 1.2 确认 `slow_threshold` 使用仓库现有 `time.Duration` decode 方式，不新增临时 duration 解析逻辑。
- [x] 1.3 更新 `user-service/internal/config/config_test.go` 和相关配置 fixture，移除 `ent.sql_debug` 断言并覆盖 `ent.plugins.*` 默认值与显式配置。

## 2. Ent 插件架构

- [x] 2.1 新增 `user-service/internal/providers/ent_plugins.go`，定义 `entDriverPlugin`、`entClientPlugin`、`entPluginSet` 和 `newEntPlugins`。
- [x] 2.2 实现 `newEntPlugins` 默认配置与启用规则：默认 tracing 开启，SQL log 和 metrics 关闭；provider 为空或 disabled 时跳过对应插件且不 panic。
- [x] 2.3 修改 `ProvideEntClients`，先构造 plugin set，再调用新签名 `newEntClient(params.PrimaryDB, plugins)`。
- [x] 2.4 修改 `newEntClient`，按顺序应用 driver plugins，再创建 `*ent.Client`，再安装 client plugins，client plugin 失败时关闭已创建 client。
- [x] 2.5 修改 `newEntDriver(db *sql.DB)`，只返回 `nonClosingEntDriver{Driver: entsql.OpenDB(dialect.Postgres, db)}`。

## 3. SQL Log 插件

- [x] 3.1 新增或拆分 `user-service/internal/providers/ent_sql_log.go`，将 `entObservabilityDriver`、`entObservabilityTx` 和 `newEntObservabilityDriver` 重命名为 SQL log 专用类型与构造函数。
- [x] 3.2 实现 `entSQLLogPlugin.WrapEntDriver`，处理 nil logger、默认慢 SQL 阈值和默认时钟。
- [x] 3.3 保留 SQL log 稳定字段和 `ent sql completed`、`ent sql slow`、`ent sql failed` message，但仅在 SQL log 插件启用后输出。
- [x] 3.4 删除 `entSQLDebugEnabled` 以及 `newEntDriver` 中的 SQL log wrapper 默认安装逻辑。

## 4. Tracing 与 Metrics 插件

- [x] 4.1 新增或拆分 `user-service/internal/providers/ent_tracing.go`，实现 `entTracingPlugin`、`installEntQueryTracing` 和 `installEntMutationTracing`。
- [x] 4.2 确保 query span 名为 `ent.query`，mutation span 名为 `ent.mutation`，并记录 `db.system=postgresql`、`ent.entity`、`ent.operation` 等低基数属性。
- [x] 4.3 确保 tracing error 路径调用 `span.RecordError(err)`、设置 error status，并通过 `defer span.End()` 避免 span 泄漏。
- [x] 4.4 新增或拆分 `user-service/internal/providers/ent_metrics.go`，实现 `entMetricsPlugin` 和 `installEntQueryMetrics`。
- [x] 4.5 保留 Ent metrics 名称 `aegiscore_user_service_ent_query_duration_seconds` 和 `aegiscore_user_service_ent_query_errors_total`，并确保 collector 注册冲突向上返回错误。
- [x] 4.6 删除旧 `installEntObservability`、`installEntQueryObservability`，并确保 tracing 与 metrics 可独立启停。

## 5. 测试覆盖

- [x] 5.1 删除或重写旧测试 `TestEntSQLDebugEnabledRequiresConfigFlag` 和 `TestNewEntDriverAlwaysObservesErrorsAndUsesConfiguredDebugLevel`。
- [x] 5.2 添加 SQL log 插件测试，覆盖启用包装、debug 成功 SQL、慢 SQL warn、SQL error 和默认不包装。
- [x] 5.3 添加插件组合测试，覆盖默认仅 tracing、显式 SQL log、显式 metrics、metrics 注册错误、driver plugin 先于 client plugin、client plugin 失败关闭 client。
- [x] 5.4 添加 tracing 插件测试，覆盖 query span、mutation span、error status 和 disabled 时不安装 span。
- [x] 5.5 添加 metrics 插件测试，覆盖 query latency、query error 和 disabled 时不注册 collector。
- [x] 5.6 保留或新增测试确认 `nonClosingEntDriver.Close()` 不关闭底层 `*sql.DB`。

## 6. 文档与部署资产

- [x] 6.1 全仓搜索 `ent.sql_debug`、`sql_debug`，同步更新 `docs/`、`deployments/`、`user-service/cmd/serve_test.go` 和其他配置示例。
- [x] 6.2 将旧配置示例替换为 `ent.plugins.sql_log.enabled=false`、`ent.plugins.sql_log.debug=false`、`ent.plugins.sql_log.slow_threshold=500ms`、`ent.plugins.tracing.enabled=true`、`ent.plugins.metrics.enabled=false`。
- [x] 6.3 检查 Prometheus alert、Grafana dashboard、runbook 和部署说明；如果依赖 Ent query metrics，明确只有 `ent.plugins.metrics.enabled=true` 且 metrics provider 启用时有数据。
- [x] 6.4 若修改 dashboard 生成源或 provisioning JSON，运行对应生成或检查命令并确认无 drift。

## 7. 验证与收尾

- [x] 7.1 运行 `go test ./user-service/internal/providers ./user-service/internal/config` 并修复失败。
- [x] 7.2 运行 `make user-service-architecture-lint` 并修复失败。
- [x] 7.3 按影响范围运行 `make user-service-test` 并修复失败。
- [x] 7.4 检查 `git diff`，确认只包含本 change 预期代码、文档、部署资产和 OpenSpec artifacts。
- [x] 7.5 将本次预期变更加到暂存区后运行 `make lint`，未通过时修复后重跑。
- [x] 7.6 保持本次预期变更处于暂存区后运行 `make verify`，未通过时修复后重跑。
- [x] 7.7 所有验证通过后，将已完成任务 checkbox 更新为 `- [x]`。
