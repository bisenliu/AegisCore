## ADDED Requirements

### Requirement: 服务健康检查

系统 MUST 暴露服务健康检查能力，用于报告 user-service runtime 及关键依赖状态。

#### Scenario: 服务健康

- **WHEN** HTTP 服务和关键依赖均可用
- **THEN** 健康检查 MUST 返回成功状态并包含服务名称或依赖检查信息

#### Scenario: 依赖异常

- **WHEN** Postgres、Redis 或其他被配置为健康检查的依赖不可用
- **THEN** 健康检查 MUST 返回失败状态并暴露可定位的依赖错误信息

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

### Requirement: Metrics 和 tracing

系统 MUST 提供 Prometheus metrics 与 OpenTelemetry tracing 基础能力，并通过共享 provider 保持服务、环境和资源标签一致。

#### Scenario: 访问 metrics

- **WHEN** metrics 配置允许暴露指标
- **THEN** user-service MUST 注册 metrics 路由，并导出 HTTP、SQL、Redis、runtime、scheduler 或 workerpool 相关指标

#### Scenario: metrics 配置禁用

- **WHEN** metrics 暴露被配置为禁用
- **THEN** 系统 MUST 不暴露 metrics 路由或返回符合配置的禁用行为

#### Scenario: tracing provider 初始化

- **WHEN** tracing 配置启用
- **THEN** 系统 MUST 初始化 OpenTelemetry provider，并使用服务名和环境标签关联 trace

### Requirement: 日志与错误可观测性

系统 MUST 使用共享 logger 和 HTTP middleware 输出结构化日志，并在错误路径记录可关联的请求、span 和业务错误信息。

#### Scenario: 请求日志

- **WHEN** HTTP 请求被处理
- **THEN** 系统 MUST 记录方法、路径、状态码、耗时和关联上下文字段

#### Scenario: panic recovery

- **WHEN** HTTP handler 或 middleware 发生 panic
- **THEN** 系统 MUST 通过 recovery middleware 捕获 panic、记录错误并返回一致错误响应

#### Scenario: span error

- **WHEN** 业务错误需要关联 tracing span
- **THEN** 系统 MUST 使用共享 span error helper 记录错误状态

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
