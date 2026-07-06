## ADDED Requirements

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
