## Context

`common/runtime/observability/metrics/status.go` 当前同时承载业务中立的 component status collector 和 Casbin policy reload recorder。前者是跨服务 runtime primitive，后者包含 `aegiscore_casbin_policy_reloads_total`、`aegiscore_casbin_policy_reload_last_success`、`ReloadMetrics`、`NopReloadMetrics` 与 `NewCasbinPolicyReloadMetrics`，属于 user-service permission/RBAC 业务语义。

permission feature 当前在 `user-service/internal/features/permission/fx.go` 中直接注册 `commonmetrics.NewCasbinPolicyReloadMetrics`，并在 `fx_authorization.go` 与 `infrastructure/casbin/enforcer.go` 中把 `commonmetrics.ReloadMetrics` 注入 Casbin Engine。迁移必须保持同一个 Engine 的 reload、授权、health 和初始化投影视图不变，也必须保持 metrics 禁用时依赖图仍获得非 nil 空实现。

本 change 不涉及 HTTP API、OpenAPI、数据库 schema、Atlas migration、dashboard、alert 阈值或 Prometheus scrape endpoint。主要影响 Go 代码、architecture lint 规则、OpenSpec 主规格 delta 和测试。

## Goals / Non-Goals

**Goals:**

- 将 Casbin reload recorder 的接口、空实现、Prometheus collector 和指标定义迁到 permission feature 或 permission-owned observability adapter。
- 从 `common/runtime/observability/metrics` 删除 Casbin、permission、role 或 user-service 业务语义，只保留通用 Provider、label、registry、HTTP metrics 支撑和 component status collector。
- 保持 `aegiscore_casbin_policy_reloads_total{status="success|failure"}` 和 `aegiscore_casbin_policy_reload_last_success` 的名称、label、计数和 gauge 语义不变。
- 使 metrics disabled 模式继续返回安全 no-op recorder，Engine、watcher、initializer 和 tests 不需要 nil 分支。
- 增加架构门禁，禁止 Casbin、permission、role 或 user-service 业务 metrics 再进入 `common/runtime/observability/metrics`。

**Non-Goals:**

- 不修改 Casbin reload、revision-aware projection、policy loader、watcher、outbox 或 user-role cache 的运行时行为。
- 不重命名 Prometheus 指标，不修改 label 枚举、dashboard 查询、alert 阈值、runbook 或 metrics endpoint 路径。
- 不把 permission metrics 放入 `user-service/internal/shared`，不新增兼容 wrapper，不在 common 和 permission 双写 recorder。
- 不引入新的外部依赖、数据库 migration 或部署资源。

## Decisions

1. Casbin reload metrics 接口由 permission feature 拥有。

   迁移后在 `user-service/internal/features/permission/application` 或 `user-service/internal/features/permission/infrastructure/casbin` 附近定义最小 recorder interface 与 no-op 实现，Engine 只依赖 permission-owned 类型。这样 application/infrastructure 不再导入 common metrics 的业务接口，业务语义不会通过类型名称反向进入 `common`。

   备选方案是把接口留在 `common`，只迁移 collector 实现。该方案仍让 common 拥有 `ReloadMetrics` 业务抽象，不能修复反向拥有关系。

2. Prometheus collector 由 permission Fx metrics adapter 注册。

   `newPermissionMetrics` 已经位于 permission feature 内并负责 RBAC policy sync、watcher、outbox 等 feature metrics。Casbin reload collector 应在同一 feature metrics 接线区域通过通用 `metrics.Provider` 注册，复用 common 的 Provider、label 常量和 disabled 判断，但指标名称和 help 文案由 permission 拥有。

   备选方案是在 `user-service/internal/providers/observability` 注册 recorder。该方案会让服务级 provider 承载 permission 细节，并增加 composition root 对 feature 内部指标的了解。

3. 指标契约保持原样。

   迁移只改变代码所有权，不改变 `aegiscore_casbin_policy_reloads_total` 的 `status` label、`success|failure` 枚举，不改变 `aegiscore_casbin_policy_reload_last_success` 的 1/0 gauge 语义。测试需要从 common runtime metrics 测试迁到 permission feature 测试，继续通过 Prometheus gather 验证指标输出。

   备选方案是重命名到 `aegiscore_user_service_rbac_*`。该方案会影响现有 PromQL、dashboard 和 alert，不符合本 change 范围。

4. architecture lint 扫描 common metrics 的业务词汇。

   在 `user-service/scripts/architecture/lint.sh` 中增加针对 `common/runtime/observability/metrics` 的静态扫描，禁止 `Casbin`、`permission`、`role`、`user-service`、`rbac` 或 `aegiscore_casbin` 等业务语义重新进入。同步更新 `architecture/lint-test.sh` fixture，避免规则漂移。

   备选方案是只依靠 code review。该方案不能形成持续门禁，无法满足验收中防回流要求。

## Risks / Trade-offs

- [Risk] 指标迁移时重复注册 collector 或遗漏注册导致 Prometheus 输出变化 -> 通过 permission metrics 单元测试覆盖 enabled 和 disabled provider，并检查 gather 输出只出现既有指标一次。
- [Risk] Engine 依赖类型迁移造成 Fx graph 缺失或命名冲突 -> 通过 permission package tests、Fx graph/bootstrap 相关测试和 `make user-service-architecture-lint` 验证。
- [Risk] 架构门禁误伤 common 中业务中立的 `role` 单词 -> 使用路径限定和较具体的 forbidden pattern，必要时只扫描 `common/runtime/observability/metrics` 的 Go 源码。
- [Risk] 迁移测试只验证 no-op 不 panic，未验证指标语义 -> 保留原 common 测试中成功、失败、last status 的 Prometheus 输出断言并迁移到 permission feature。

## Migration Plan

1. 在 permission feature 中新增 Casbin reload recorder interface、no-op 和 Prometheus implementation。
2. 修改 `permissionMetricsOptions` 改为提供 permission-owned recorder，移除 `commonmetrics.NewCasbinPolicyReloadMetrics` 注册。
3. 修改 Casbin Engine constructor、mockgen source、测试和 e2e fixture，全部使用 permission-owned recorder interface 与 no-op。
4. 从 `common/runtime/observability/metrics/status.go` 删除 Casbin reload metrics 相关常量、接口、实现和 constructor，保留 component status collector。
5. 将原 common reload metrics 测试迁移到 permission feature，删除 common 中对应测试。
6. 增加 architecture lint 和 fixture 测试，阻止业务 metrics 回流 common。
7. 运行 permission 相关包测试、common runtime metrics 测试、`make user-service-architecture-lint`、`make lint` 和 `make verify`。

回滚策略是恢复迁移前的 common recorder 并恢复 permission Fx 接线，但由于本 change 不改变外部指标或数据结构，正常发布无需运行数据迁移或双写窗口。

## Open Questions

无。指标所有权、名称保留和不修改运行时行为的边界已由本 change 固定。
