## Why

`common/runtime` 中部分 Fx provider 和 process runtime 初始化入口的命名与职责边界不够清晰，容易让跨服务 primitive 暴露出偏服务装配层的 API。现在需要在不改变运行时行为的前提下，评估并收敛 common 层 Fx provider 命名、timezone 初始化归属，以及 user-service composition root 中的 lifecycle 绑定方式。

## What Changes

- 评估并调整 `common/runtime/observability/metrics` 和 `common/runtime/observability/tracing` 中公开 Fx provider 的命名，使其表达 metrics/tracing runtime 能力，而不是泛化的 `NewFxProvider`。
- 评估 `common/runtime/timezone` 的 Fx 包装是否仍有真实跨服务价值；如果仅包装 `Init`，则改为由 user-service composition root 显式调用 timezone 初始化。
- 调整 `user-service/internal/bootstrap` 和 `user-service/internal/providers` 的 Fx module 组合，使 process runtime 初始化、observability provider 和业务 lifecycle module 的启动顺序由服务组合层明确表达。
- 保持 metrics、tracing、timezone 的现有运行时语义、配置契约、OpenAPI/API、数据库 schema 和部署资产不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 明确 common runtime primitive 和 Fx adapter 的公开 API 命名与 lifecycle 归属边界。
- `runtime-observability`: 明确 metrics/tracing provider 在 Fx lifecycle 中的命名、启停和服务 composition root 绑定要求。

## Impact

- 受影响代码：`common/runtime/timezone/fx.go`、`common/runtime/timezone/timezone.go`、`common/runtime/observability/metrics/fx.go`、`common/runtime/observability/tracing/fx.go`、`user-service/internal/bootstrap/app.go`、`user-service/internal/providers/fx.go`。
- 可能受影响测试：common runtime observability/timezone 测试、user-service Fx app 构图和启动测试、架构边界 lint。
- 不改变 HTTP API、OpenAPI 生成物、Ent schema、Atlas migration、Redis/PostgreSQL 资源配置、metrics family、tracing exporter 配置或部署观测资产。
