## Context

本变更聚焦 user-service RBAC feature 的 Fx composition 表达。当前 permission composition 中存在多个只做 concrete-to-interface identity projection 的 helper，以及只转发 metrics constructor 的 wrapper；role 和 permission transport 中也存在只包含单个普通依赖的 Fx Params。这些结构没有业务语义、配置转换、错误包装或 lifecycle 职责，却让 RBAC graph 的真实实例身份和依赖边变得不直观。

RBAC 当前依赖仍然需要同时满足 concrete 与 interface 消费：`*permissioncasbin.Engine` 被初始 policy load、health check、授权服务和 policy refresh coordinator 消费；`*permissionredis.Store` 既是 Redis store concrete，也是 policy version publisher；`*permissionredis.VersionTracker` 是 watcher 输入并提供 tracker port；`*permissionredis.Watcher` 需要作为 concrete 被显式 invoke 以注册 lifecycle，同时提供 watcher status。composition 标准化必须保留这些实例身份和启动停止语义。

## Goals / Non-Goals

**Goals:**

- 使用 Fx 原生 `fx.Annotate`、`fx.As` 和 `fx.Self` 明确表达同一 constructor 同时提供 concrete 与多个 interface 视图。
- 删除无业务语义的 projection helper、constructor forwarding 和单字段普通 Fx Params。
- 将 Casbin reload metrics 从 optional 输入改为正式 graph 必选输入；metrics 禁用时由既有 metrics constructor 提供 `NopReloadMetrics()`。
- 保持 RBAC 权限目录、授权、policy reload、Redis policy version、Pub/Sub watcher、health、metrics 和 lifecycle 行为不变。
- 通过直接构造测试和 module graph 测试验证同实例投影、必需依赖和启动停止能力。

**Non-Goals:**

- 不改变权限目录、route diff、角色、角色权限、用户角色、Casbin reload、Redis policy version、Pub/Sub、watcher 补偿或授权结果。
- 不改变 HTTP API、OpenAPI、Ent schema、Atlas migration、配置、部署资产、metrics 名称或日志字段。
- 不创建通用 reflection helper、全局 DI facade、无业务意义的大接口或兼容 wrapper。
- 不把 user-service RBAC composition 逻辑移动到 `common` 或 `internal/shared`。
- 不修改 application port 所有权，不合并接口，不移动 `PolicyChangeNotifier`、`WatcherStatus` 或 RBAC policy sync 模型。
- 不并行修改 auth command use case 的重复 Params、历史 optional metrics 或 credential verifier 输入适配；这些由独立 `simplify-auth-command-composition` change 处理。
- 不扩展为全面移除 infrastructure/transport Fx metadata；具名 Ent、Redis、worker pool、cache 依赖使用的 Fx Params/tag 继续保留。
- 不移除 `cmd.newBootstrapLifecycleApp` 测试 seam，它不属于 Fx provider identity projection。

## Decisions

- 决策：使用 `fx.Annotate(constructor, fx.As(fx.Self()), fx.As(new(...)))` 表达需要保留 concrete 的 provider projection。
  理由：`fx.As` 会替换默认输出类型，因此仍有 concrete 消费方的 constructor 必须同时声明 `fx.As(fx.Self())`。这样可以由单次 constructor 调用提供全部视图，避免重复注册 constructor 导致重复 Engine、Store、Tracker 或 Watcher 实例。
  备选方案：保留 identity helper。该方案虽然简单，但继续隐藏 projection 关系并增加无语义函数。备选方案：只用 `fx.As(interface)`。该方案会丢失 concrete 输出，破坏初始 policy load、health check、watcher 输入或显式 invoke。

- 决策：interface projection 必须紧邻其 concrete constructor 声明，不再通过散落的 identity helper 隐式表达 graph edge。
  理由：RBAC graph 的关键约束是同一实例同时暴露多个视图，projection 放在 constructor 注册处最容易审查实例身份和消费关系。
  备选方案：在单独文件集中声明 projection helper。该方案仍会把实例身份和 constructor 来源拆开，降低 graph 可读性。

- 决策：`*permissioncasbin.Engine` 必须同时提供 concrete、`permissionauthorization.Engine` 和 `permissionapplication.PolicyReloadEngine`。
  理由：concrete 消费方包括初始 policy load 和 health check；interface 消费方包括授权服务和 policy refresh coordinator。`fx.Self` 不可省略。
  备选方案：拆分 Engine wrapper 或增加 adapter。该方案会引入无业务意义类型，并可能改变错误、metrics 或 reload 语义。

- 决策：`*permissionredis.Store`、`*permissionredis.VersionTracker` 和 `*permissionredis.Watcher` 均保留 concrete 输出，并分别投影到所需 interface。
  理由：`*Store` 和 `*VersionTracker` 是 watcher 输入；`*Watcher` 仍被显式 invoke 以保证 lifecycle 注册和启停语义。这些 Self 投影不可省略。
  备选方案：只保留 interface 并调整 watcher 入参。该方案会扩大 port 边界并把具体 Redis 协作关系伪装成 application 抽象，不属于本变更目标。

- 决策：直接注册 `commonmetrics.NewCasbinPolicyReloadMetrics`，并将 Casbin Engine metrics 输入改为必选。
  理由：`newCasbinReloadMetrics` 没有附加逻辑；正式 permission module 始终注册 metrics provider。metrics 禁用属于 provider 内部配置语义，应通过 `NopReloadMetrics()` 表达，而不是通过缺失依赖 optional 降级表达。
  备选方案：继续使用 optional 输入。该方案使 graph 在 provider 缺失时静默降级，不利于发现 composition 缺陷。

- 决策：直接注册已经返回 `permissionauthorization.Authorizer` 的 `permissionauthorization.NewAuthorizer`。
  理由：外层 `fx.As(new(permissionauthorization.Authorizer))` 不改变输出类型，属于冗余 annotation。
  备选方案：保留 annotation。该方案没有行为收益，只增加误导性 graph metadata。

- 决策：单字段、无 `name`、`optional` 或 group tag 的 Fx Params 改为普通参数。
  理由：`PermissionLookupParams` 与 `RouteCatalogScannerParams` 只是包装一个普通依赖，直接参数更利于构造器单元测试，也不改变 Fx graph。
  备选方案：保留 Fx Params 以便未来扩展。该方案属于推测性兼容；若未来需要 metadata，可在真实需求出现时再引入。

## Risks / Trade-offs

- 风险：遗漏 `fx.Self()` 导致 concrete 消费方无法解析。缓解：module graph 测试同时解析 concrete 和 interface，并覆盖初始 load、health/metrics consumer、watcher 显式实例化。
- 风险：重复注册 constructor 造成多个 Engine、Store、Tracker 或 Watcher 实例。缓解：测试断言 concrete 与 interface projection 指向同一实例，并审计 provider 列表确保每个 constructor 只执行一次。
- 风险：metrics 输入从 optional 改为必选后，测试 graph 如果未注册 metrics provider 会失败。缓解：直接构造测试显式传入 `commonmetrics.NopReloadMetrics()`；正式 module 测试验证缺少 reload metrics provider 时构图失败。
- 风险：删除单字段 Fx Params 可能影响直接测试和少量调用点。缓解：同步更新 constructor 调用和测试 fixture，不改变 application port 或 transport 行为。
- 取舍：本变更提高 graph 显式性，但仍保留需要 DI metadata 的 Fx Params/tag，避免把局部整理扩大成不必要的架构迁移。

## Migration Plan

- 这是内部 composition 和测试结构调整，不涉及数据迁移、OpenAPI 生成、部署资产或配置迁移。
- 实施顺序为先审计 composition helper 和 Fx Params，再修改 permission provider 注册，然后调整 direct constructor 与 module graph 测试。
- 回滚策略是恢复原有 identity helper、optional metrics 输入和单字段 Fx Params；由于不改变持久化数据和外部契约，回滚不需要数据修复。
- 验证方式包括相关 Go 包测试、architecture lint、OpenSpec validate、全仓 lint 和 verify。

## Open Questions

- 无待决问题。auth composition 的相似清理范围已明确排除，由独立 change 处理。
