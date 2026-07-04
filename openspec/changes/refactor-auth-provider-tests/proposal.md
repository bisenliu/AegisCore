## Why

认证与 provider 相关测试仍存在多个 500 行以上的大文件，且部分文件混合 session/token、login/refresh/logout、Gin runtime、route 注册和 metrics 等不同子主题，影响定位、并行维护和代码评审。当前 `mockgen` 与 metrics no-op 生成已大量落地，但生成约定和少量复杂手写 fake/stub 仍散落在测试代码中，需要用一次性重构消除残留维护成本。

## What Changes

- **BREAKING**: 不保留旧的大型测试文件布局；按子主题拆分 auth Redis session store、auth command service、provider routes 和 provider Gin engine 测试文件。
- **BREAKING**: 不保留复杂手写 collaborator fake/stub 兼容路径；可由 `mockgen` 表达的外部端口交互必须迁移为生成 mock expectation。
- 统一 feature-local metrics no-op 的生成约定，继续保留 `common/runtime/observability/metrics/nopgen` 作为业务中立生成器，不把 auth/permission 业务指标方法上提到 `common`。
- 将轻量纯构造 helper、领域值 fixture、真实 Redis/miniredis 测试夹具集中到对应主题的 `*_test_helpers_test.go`，避免跨主题测试共享隐式状态。
- 保持现有业务 API、数据库 schema、OpenAPI、运行时 metrics/tracing/logging 行为不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: 增加 Go 测试组织、复杂测试替身和生成物约定，要求认证与 provider 测试按主题拆分，复杂外部 collaborator 使用生成 mock，不保留旧兼容布局。

## Impact

- 影响测试文件组织：`user-service/internal/features/auth/infrastructure/redis/`、`user-service/internal/features/auth/application/command/`、`user-service/internal/providers/`。
- 影响测试替身维护：复杂 fake/stub/spy 迁移到 `mockgen` 生成物或主题内 helper；轻量值对象 fixture 仍可作为 `_test.go` helper 保留。
- 影响生成约定：继续使用 `go.uber.org/mock/mockgen` 和 `common/runtime/observability/metrics/nopgen`，并通过生成与 verify 暴露 drift。
- 不影响 HTTP API、OpenAPI 契约、Ent schema、Atlas migration、部署资产、Prometheus metric family、OpenTelemetry tracing 或 RBAC 授权行为。
