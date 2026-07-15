## Why

`PermissionQueryService` 当前通过 variadic 参数接收 `permissionapplication.Metrics`，Dig/Fx 正式依赖图不会把该参数建模为明确的单值输入，导致 route diff 查询即使在 metrics 已启用时也可能持续使用 no-op 记录器，相关指标无法反映真实执行结果。需要收紧构造契约，并保证 metrics 启用和禁用两种正式 App 配置都提供确定的实现，使 route diff 指标调用可被依赖图和模块级测试验证。

## What Changes

- 将 permission route diff 查询服务的 metrics 依赖定义为正式依赖图中的必选单值输入，消除 variadic 注入造成的隐式缺失。
- metrics 启用时向 permission 模块提供真实 Prometheus 实现，禁用时提供现有 `NopMetrics()` 实现，确保两种配置都能完整构图。
- 增加从正式 `permission.Module` 构图的测试，注入 spy `permissionapplication.Metrics`，执行 route diff 查询并验证 `RouteDiffObserved` 被调用。
- 规定 route diff 诊断完成后必须把 missing 与 stale 数量交给当前配置选择的 metrics 实现，同时保持既有业务判定和无副作用语义。
- 不修改 HTTP response、权限目录、Casbin policy、指标名称、metrics backend 或 dashboard，也不重组 permission feature 的其他 Fx provider。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`: 明确正式 permission 模块执行 route diff 时必须调用已注入的 metrics 实现，并以模块级测试固定该协作者契约。
- `runtime-observability`: 明确 metrics 启用和禁用配置都必须向 permission 正式依赖图提供单值 `Metrics` 实现，且禁用时使用现有 no-op 实现。

## Impact

- 受影响代码：`user-service/internal/features/permission/application/query/`、`user-service/internal/features/permission/fx.go`、`user-service/internal/features/permission/metrics.go` 及对应测试。
- 依赖注入：`PermissionQueryService` 的 Fx/Dig 图新增明确的 `permissionapplication.Metrics` 输入边；metrics 禁用配置继续通过 `NopMetrics()` 保持无副作用。
- 规格：更新 `rbac-access-control` 与 `runtime-observability` 的 route diff 指标真实性要求。
- API、HTTP response、数据库 schema、OpenAPI、Casbin policy、部署资产、指标名称和 dashboard 均不变；不引入新依赖或新 metrics backend。
