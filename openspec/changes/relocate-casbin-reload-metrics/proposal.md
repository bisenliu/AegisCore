## Why

`common/runtime/observability/metrics` 当前拥有 Casbin policy reload 指标、接口和空实现，使跨服务 runtime metrics 反向承载了 user-service RBAC 业务语义。该边界会让 `common` 继续吸收 permission、role 或 user-service 专属指标，削弱共享 primitive 的业务中立性。

## What Changes

- 将 Casbin policy reload 指标 recorder、空实现和 `aegiscore_casbin_*` 指标定义从 `common/runtime/observability/metrics` 迁回 user-service permission feature 或其 observability adapter。
- 保留通用 metrics Provider、Prometheus registry、HTTP metrics endpoint 与 component status collector 的跨服务边界。
- 保持现有 Prometheus 指标名称、label、计数器和 gauge 语义不变，disabled metrics 仍返回安全空实现。
- 更新 Fx 接线、permission Engine 依赖和相关测试，使 RBAC policy reload 继续记录相同采集结果。
- 补充架构门禁，防止 Casbin、permission、role 或 user-service 业务指标重新进入 `common/runtime/observability/metrics`。
- 不改变 Casbin reload、revision、projection、watcher、dashboard、alert 阈值或 HTTP metrics endpoint 的运行时行为。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 明确 `common/runtime/observability/metrics` 只能承载业务中立的共享 metrics primitive，不拥有 Casbin、permission、role 或 user-service 专属业务指标。
- `rbac-access-control`: 明确 Casbin policy reload 指标 recorder 属于 permission/RBAC 边界，迁移后保持指标名称、label 和采集语义不变。
- `runtime-observability`: 明确通用 metrics Provider、component status collector 和 metrics endpoint 保持跨服务共享能力，feature metrics 由 feature 或服务级 adapter 拥有并注册。

## Impact

- 影响 `common/runtime/observability/metrics` 中的 Casbin reload recorder、空实现和指标定义。
- 影响 `user-service/internal/features/permission/` 或 permission observability adapter 的 recorder 定义、Fx provider 接线、Engine 构造参数和测试。
- 影响 runtime dependency 或 architecture lint 规则，用于阻止 user-service 业务指标重新进入 `common`。
- 不影响外部 HTTP API、OpenAPI、数据库 schema、migration、Prometheus 指标名称、label、dashboard、alert 阈值或 metrics endpoint 路径。
