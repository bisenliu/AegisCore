## Context

`user-service/internal/features/permission/application/query/NewPermissionQueryService` 当前以 `...permissionapplication.Metrics` 接收可选指标记录器，并在未传入时回退到 `NopMetrics()`。该模式适合手工调用，但 Dig/Fx 不把 variadic 参数作为普通单值依赖注入，因此 `user-service/internal/features/permission/fx.go` 虽已通过 `newPermissionMetrics` 提供 `permissionapplication.Metrics`，query service 在正式 App 图中仍没有对应输入边，route diff 的 `RouteDiffObserved` 调用最终落到构造器内部的 no-op。

`newPermissionMetrics` 已实现所需的配置选择：metrics provider 启用时创建并注册 permission Prometheus recorder，禁用或 provider 不可用时返回 feature-local `permissionapplication.NopMetrics()`。本次变更应复用该实现，只修复 query service 构造契约与正式 Fx 接线，并用正式 `permission.Module` 测试证明指标协作者可到达。

变更归属 `user-service/internal/features/permission/`。`common/` 继续只提供业务中立的 metrics provider 和 no-op 生成能力；`deployments/`、dashboard 与告警不变；`docs/openspec` 只更新 `rbac-access-control` 和 `runtime-observability` 的稳定行为约束。

## Goals / Non-Goals

**Goals:**

- 让 `PermissionQueryService` 在 Fx/Dig 图中具有明确且必选的单值 `permissionapplication.Metrics` 输入边。
- metrics 启用时使用现有 Prometheus recorder，禁用时使用现有 `NopMetrics()`，两种配置都可完成正式 App 构图。
- 从正式 `permission.Module` 注入 spy Metrics，执行 route diff 并观察到携带准确 missing、stale 数量的 `RouteDiffObserved` 调用。
- 保持 route diff 计算、只读语义、错误传播和现有 metric family/label 契约不变。

**Non-Goals:**

- 不修改 route diff 的 missing、stale、mismatch 判定、排序或路由过滤规则。
- 不修改 permission HTTP response、权限目录、角色绑定或 Casbin policy/policy sync 语义。
- 不新增 metrics backend、指标名称、label、dashboard、alert 或部署清单。
- 不重组 permission feature 的全部 Fx provider，不把业务 Metrics interface 或 adapter 移入 `common/`、`internal/shared/` 或 `internal/integration/`。
- 不涉及数据库 schema、Atlas migration、OpenAPI 生成物或安全边界变更。

## Decisions

### Decision: application 构造器直接要求单值 Metrics

将 `NewPermissionQueryService` 的第三个参数改为必选的 `permissionapplication.Metrics`，删除 variadic 选择与构造器内部 no-op fallback。所有纯 application 调用点显式传入测试 spy/mock 或 `permissionapplication.NopMetrics()`。

选择该方案是因为依赖要求在消费侧最清晰，Go 调用点和 Dig/Fx 都能静态表达同一契约，且不会把生产接线规则隐藏在 adapter 中。备选方案是在 `features/permission/fx.go` 新增非 variadic wrapper，仅供 Fx 调用并保留旧 application 构造器；该方案会维持两种构造语义和不必要的兼容层，当前仓库调用点均在本 feature 内，无兼容收益，因此不采用。也不使用 `optional:"true"`、slice/group annotation 或额外 variadic adapter，因为它们会继续允许正式图静默缺失指标实现。

### Decision: 复用现有配置选择 provider

保留 `newPermissionMetrics(*commonmetrics.Provider) (permissionapplication.Metrics, error)` 作为正式 module 中唯一的 feature Metrics provider。provider 启用时返回 `*prometheusMetrics`，禁用时返回 `permissionapplication.NopMetrics()`；query service 只消费接口，不读取配置或判断实现类型。

该选择保持 feature-local 指标归属，并避免将业务指标逻辑下沉到 `common/runtime/observability/metrics`。备选方案是让 query service 接收 `*commonmetrics.Provider` 后自行选择实现，但会破坏 application 对 runtime infrastructure 的隔离，因此不采用。

### Decision: 以正式 Module 的可观察行为和 DOT 图固定接线

在 permission 根包增加模块级测试，使用正式 `permission.Module`，替换或装饰其 `permissionapplication.Metrics` 输出为 spy，并为 module 提供最小测试依赖。测试从 Fx 容器取得 `permissionquery.PermissionQueryService`，执行 `GetRouteDiff`，断言 spy 收到一次准确的 `RouteDiffObserved` 调用；同时取得 `fx.DotGraph` 或等价可视化结果，断言 query service 构造器具有明确的 Metrics 输入边。

另以配置化 provider 测试覆盖 metrics 启用和禁用时的构图，禁用分支断言得到的接口为现有 no-op 行为。相比只测试 `newPermissionMetrics` 或只对 application 构造器做单元测试，正式 module 测试可以直接捕获本次 Dig/Fx variadic 注入回归。

### Decision: route diff 成功后记录当前结果

保留 `GetRouteDiff` 在成功完成扫描、目录读取和差异计算后调用 `RouteDiffObserved(ctx, len(missing), len(stale))` 的时机。前置步骤返回错误时不伪造成功快照；指标调用不改变返回结果，也不执行任何目录或 policy 写操作。

## Risks / Trade-offs

- [Risk] 构造器签名收紧会使遗漏 Metrics 的纯 application 测试无法编译。 -> Mitigation：一次性更新 feature 内全部调用点；无指标断言的测试显式使用 `permissionapplication.NopMetrics()`，需要验证协作者的测试继续传入 mock/spy。
- [Risk] 正式 `permission.Module` 包含 Casbin、Redis watcher 和 lifecycle provider，模块级测试准备依赖较多且可能脆弱。 -> Mitigation：复用当前 port 边界提供最小 fake，并只替换 Metrics 输出；不复制生产 provider 列表或创建仅供测试的生产分支。
- [Risk] Fx replacement/decorator 使用错误类型可能绕过正式 `newPermissionMetrics` 或形成重复 provider。 -> Mitigation：测试按 `permissionapplication.Metrics` 的精确接口类型替换/装饰，并同时检查构图和实际 route diff 调用。
- [Trade-off] 必选 Metrics 增加手工构造时的一个参数，但换来消费契约显式、正式图可验证，且 `NopMetrics()` 已提供低成本默认实现。

## Migration Plan

1. 收紧 query service 构造签名并更新 feature 内全部直接调用点。
2. 保持 `newPermissionMetrics` 的 enabled/disabled 选择逻辑，确认正式 Fx provider 只产生一个 `permissionapplication.Metrics`。
3. 增加正式 module spy/DOT 测试以及 enabled/disabled 构图覆盖，运行 permission feature 测试。
4. 验证 OpenSpec 与架构规则；实现全部完成并暂存预期变更后运行仓库 lint 和 verify。

本次不需要数据或发布迁移，可随 user-service 常规发布生效。回滚时恢复构造器与 Fx 接线及对应测试、规格 delta 即可；不存在数据库、API 或观测资产回滚步骤。

验证命令：

- `cd user-service && go test ./internal/features/permission/... -count=1`
- `openspec validate require-permission-route-diff-metrics`
- `make user-service-architecture-lint`
- 暂存本次预期变更后执行 `make lint`
- 暂存本次预期变更后执行 `make verify`

## Open Questions

无。
