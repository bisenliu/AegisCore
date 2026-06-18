## Purpose

定义 user-service 和共享 runtime 的可观测性能力，覆盖健康检查、OpenAPI、metrics、tracing、日志、错误可观测性和部署观测资产。

## Requirements

### Requirement: 服务健康检查

系统 MUST 暴露 `/livez`、`/readyz` 和 `/startupz` 健康检查能力，用于报告 user-service runtime 及关键依赖状态。

#### Scenario: 服务健康

- **WHEN** HTTP 服务和关键依赖均可用
- **THEN** 健康检查 MUST 返回成功状态并包含服务名称或依赖检查信息

#### Scenario: 存活检查

- **WHEN** 调用 `/livez`
- **THEN** endpoint MUST 只证明进程可响应，并 MAY 在 PostgreSQL、Redis 或 RBAC policy 状态异常时继续返回成功

#### Scenario: 依赖异常

- **WHEN** PostgreSQL、Redis、Casbin policy 或 RBAC policy watcher 等被配置为就绪或启动检查的依赖不可用
- **THEN** `/readyz` 或 `/startupz` MUST 返回失败状态并暴露可定位的依赖错误信息，且 MUST NOT 暴露 secret、stacktrace、DSN、SQL、token 或 Cookie

#### Scenario: 路由注册

- **WHEN** user-service 启动并注册 HTTP 路由
- **THEN** 健康检查路由 MUST 在业务 API 外可访问，并不依赖业务授权中间件

### Requirement: OpenAPI 文档

系统 MUST 暴露和生成 OpenAPI 3 文档，覆盖认证会话、用户资料、角色管理、权限目录、RBAC 授权保护接口和健康检查。

#### Scenario: 访问 OpenAPI

- **WHEN** 调用方访问 OpenAPI 文档路由
- **THEN** 系统 MUST 返回与当前 user-service HTTP API 匹配的 OpenAPI 内容

#### Scenario: 生成 OpenAPI 文件

- **WHEN** 执行 `make user-service-openapi-generate`
- **THEN** 系统 MUST 更新 `user-service/docs/openapi.json`、`user-service/docs/openapi.yaml` 和相关生成文件

#### Scenario: OpenAPI drift

- **WHEN** API 注解或路由行为变化但 OpenAPI 生成物未同步
- **THEN** `make verify` MUST 能通过生成后 `git diff --exit-code` 暴露 drift

#### Scenario: 运行时文档路由归属

- **WHEN** user-service 暴露 OpenAPI UI、JSON 或 docs redirect
- **THEN** 路由 MUST 由 `user-service/internal/router/openapi.go` 拥有，且健康检查或 metrics endpoint MUST NOT 被当作 `/api/v1` 下的 feature 业务 API

### Requirement: Metrics 和 tracing

系统 MUST 提供 Prometheus metrics 与 OpenTelemetry tracing 基础能力，并通过共享 provider 保持服务、环境和资源标签一致。

#### Scenario: 访问 metrics

- **WHEN** metrics 配置允许暴露指标
- **THEN** user-service MUST 在 `/api/v1` 外注册配置化 metrics 路由，并导出 HTTP、SQL、Redis、runtime、scheduler 或 workerpool 相关指标；metrics 路由 MUST NOT 经过 RBAC 授权

#### Scenario: metrics 配置禁用

- **WHEN** metrics 暴露被配置为禁用
- **THEN** 系统 MUST 不暴露 metrics 路由或返回符合配置的禁用行为

#### Scenario: metrics 标签

- **WHEN** 系统记录 metrics 标签
- **THEN** 标签 MUST 保持低基数，MUST NOT 包含用户 ID、角色 ID、权限 ID、会话 ID、token ID、trace/span ID、raw path、IP、邮箱、用户名、SQL、Redis key 或原始错误

#### Scenario: tracing provider 初始化

- **WHEN** tracing 配置启用
- **THEN** 系统 MUST 初始化 OpenTelemetry provider，并使用服务名和环境标签关联 trace

#### Scenario: trace 上下文传播

- **WHEN** HTTP 请求携带 W3C `traceparent` 或 `tracestate`
- **THEN** 系统 MUST 使用 OpenTelemetry 上下文传播；日志 helper MUST 只从有效 span context 派生 `trace_id` 和 `span_id`，无有效 span context 时 MUST 省略这些字段

### Requirement: 日志与错误可观测性

系统 MUST 使用共享 logger 和 HTTP middleware 输出结构化日志，并在错误路径记录可关联的请求、span 和业务错误信息。

#### Scenario: 请求日志

- **WHEN** HTTP 请求被处理
- **THEN** 系统 MUST 记录方法、路径、状态码、耗时和关联上下文字段，日志字段名 MUST 使用稳定英文 `snake_case`，log message MUST 使用英文

#### Scenario: panic recovery

- **WHEN** HTTP handler 或 middleware 发生 panic
- **THEN** 系统 MUST 通过 recovery middleware 捕获 panic、记录错误并返回一致错误响应

#### Scenario: span error

- **WHEN** 业务错误需要关联 tracing span
- **THEN** 系统 MUST 使用共享 span error helper 记录错误状态

#### Scenario: 敏感日志

- **WHEN** 记录认证失败、请求错误或系统异常
- **THEN** 日志和 span event MUST NOT 记录 password、token、Authorization header、Cookie、原始请求体、DSN、SQL、Redis key 或敏感原始错误

#### Scenario: 日志等级

- **WHEN** 发生预期业务拒绝
- **THEN** 日志 MUST NOT 使用 `Error` 级别；当发生系统异常、外部依赖失败、后台任务失败或 panic recover 时，日志 MUST NOT 降级为 `Info`

### Requirement: 部署观测资产

系统 MUST 维护 Prometheus alerts、Grafana dashboards 和 Compose/Kubernetes/Helm 观测配置，使本地和集群环境能够查看 user-service 运行状态。

#### Scenario: 本地 Compose 观测

- **WHEN** 使用 `deployments/compose` 启动本地观测环境
- **THEN** Prometheus 和 Grafana MUST 能加载 user-service 相关 scrape、dashboard 和 datasource 配置

#### Scenario: dashboard 生成

- **WHEN** 执行 `make compose-dashboard-generate`
- **THEN** 系统 MUST 从通用观测 dashboard 生成 Compose Grafana dashboard

#### Scenario: dashboard drift 检查

- **WHEN** 通用 dashboard 和 Compose dashboard 不一致
- **THEN** `make compose-dashboard-check` MUST 能报告 drift
