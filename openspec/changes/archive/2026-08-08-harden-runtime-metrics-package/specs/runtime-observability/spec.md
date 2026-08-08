## ADDED Requirements

### Requirement: Metrics provider registry 与 scrape context 契约

系统 MUST 通过 `common/runtime/observability/metrics` 提供显式 enabled/disabled 状态的非 nil provider，并为启用状态使用独立 Prometheus registry。provider MUST 支持重复注册幂等、context-aware gather 和基于 HTTP request context 的 scrape handler；禁用状态 MUST 保持 no-op 且不得暴露 collector 或 HTTP metrics 输出。

#### Scenario: 启用 provider 使用独立 registry

- **WHEN** metrics provider 基于启用配置创建
- **THEN** provider MUST 返回 `Enabled()=true`，并使用独立 registry、registerer 和 gatherer
- **AND** provider MUST NOT 注册或依赖 Prometheus global registry
- **AND** service 与 environment label MUST 继续由 provider registerer 统一包装为稳定低基数字段

#### Scenario: 禁用 provider 保持正式依赖图可用

- **WHEN** metrics provider 基于禁用配置创建
- **THEN** provider MUST 返回非 nil provider 且 `Enabled()=false`
- **AND** `Registerer()` 与 `Gatherer()` MUST 返回 nil，`Register` 和 `MustRegister` MUST 保持 no-op
- **AND** `HTTPHandler` MUST NOT 暴露 metrics 内容

#### Scenario: 重复注册不破坏启动

- **WHEN** 同一 collector 或等价 collector 被重复注册到启用 provider
- **THEN** provider MUST 将 Prometheus `AlreadyRegisteredError` 视为成功
- **AND** 其他注册错误 MUST 继续向调用方返回，nil collector MUST 返回稳定错误

#### Scenario: HTTP scrape context 传播给 context-aware collector

- **WHEN** 调用方通过 `HTTPHandler` 暴露 metrics endpoint 且 HTTP request context 被取消
- **THEN** provider MUST 通过 `GatherContext` 将该 request context 提供给实现 `ContextCollector` 的 collector
- **AND** Redis PING 等支持 context 的 collector MUST 能在 scrape 取消时尽快终止
- **AND** 标准 `Collect` 或 `Gatherer().Gather()` 直接调用 MUST 使用 background context，MUST NOT 声称感知 HTTP request cancellation

#### Scenario: metrics label 保持低基数

- **WHEN** runtime collector、feature metrics 或自定义 collector 通过 provider 注册并导出指标
- **THEN** label MUST 只使用固定资源名、结果、状态、reason 或 service/environment 等低基数字段
- **AND** label MUST NOT 包含用户、角色、权限、会话、token、trace/span ID、raw path、IP、邮箱、用户名、SQL、Redis key、原始错误或其他高基数字段

### Requirement: Metrics package 文档与示例

`common/runtime/observability/metrics` MUST 提供 package 文档和可执行示例，说明 provider 启停、独立 registry、重复注册、`HTTPHandler`、`GatherContext`、collector context 和 label cardinality 的稳定用法。示例 MUST 使用本地 registry 与内存 collector，MUST NOT 访问公网或真实 datastore。

#### Scenario: go doc 导航到主要示例

- **WHEN** 开发者查看 `common/runtime/observability/metrics` 的 package 文档
- **THEN** 文档 MUST 能说明 enabled/disabled provider、独立 registry、重复注册、`HTTPHandler`、`GatherContext`、collector context 和 label cardinality 约束
- **AND** go doc MUST 能导航到主要 executable examples

#### Scenario: 示例测试不依赖外部系统

- **WHEN** 执行 metrics package 的示例测试
- **THEN** 示例 MUST 只使用本地 registry、自定义 collector、`httptest` 或等价内存对象
- **AND** 示例 MUST NOT 访问公网、PostgreSQL、Redis、scheduler、workerpool 或真实 localcache datastore
