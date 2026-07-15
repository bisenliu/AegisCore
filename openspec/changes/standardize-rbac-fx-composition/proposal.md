## Why

user-service 的 RBAC feature 目前在 Fx composition 中存在多处无业务语义的 identity projection helper、单字段 Fx Params 和纯转发 constructor，使依赖图表达分散且容易误导读者以为存在额外配置转换、生命周期或降级语义。需要标准化 permission/role 相关 provider 注册方式，在保持现有 RBAC 行为、实例身份和依赖图完整性的前提下，让 concrete 与 interface 视图的关系直接在 composition 中显式表达。

当前现状包括：

- `user-service/internal/features/permission/fx.go` 中 `newAuthorizationEngine`、`newPolicyReloadEngine`、`newPolicyVersionPublisher`、`newPolicyVersionTracker` 和 `newPolicyWatcherStatus` 只把同一个 concrete 返回为 interface。
- `newCasbinReloadMetrics` 只转发到 `commonmetrics.NewCasbinPolicyReloadMetrics`，没有额外校验、配置转换、错误包装或生命周期逻辑。
- `permissionauthorization.NewAuthorizer` 已直接返回 `permissionauthorization.Authorizer`，外层再次使用 `fx.As(new(permissionauthorization.Authorizer))` 没有改变输出类型。
- `permissioncasbin.Params.Metrics` 标记为 optional，但 permission composition 始终注册 `commonmetrics.NewCasbinPolicyReloadMetrics`；metrics provider 禁用时该 constructor 仍返回 `NopReloadMetrics()`，因此正式 graph 不需要通过缺失依赖降级。
- `role/infrastructure/postgres.PermissionLookupParams` 与 `permission/transport/http.RouteCatalogScannerParams` 都只包含一个无 `name`、`optional` 或 group tag 的普通依赖，导致构造器和直接单元测试无必要地依赖 `fx.In`。
- `permissioncasbin.Engine`、`permissionredis.Store`、`permissionredis.VersionTracker` 和 `permissionredis.Watcher` 仍有 concrete 消费方，不能只使用 `fx.As(interface)` 后丢失原始类型。

## What Changes

- 在 permission feature composition 中直接注册 `commonmetrics.NewCasbinPolicyReloadMetrics`，删除无语义 metrics 转发函数。
- 直接注册已经返回目标 interface 的 `permissionauthorization.NewAuthorizer`，删除对相同返回类型的冗余 `fx.As`。
- 将 Casbin Engine 的 `commonmetrics.ReloadMetrics` 改为正式 graph 必选输入，删除 `optional:"true"`；metrics 禁用时继续注入 `NopReloadMetrics()`，缺少 reload metrics provider 时必须构图失败。
- 使用 Fx v1.24 已支持的 `fx.Annotate`、`fx.As` 和 `fx.Self`，让同一个 `*permissioncasbin.Engine` 同时作为 concrete、`permissionauthorization.Engine` 和 `permissionapplication.PolicyReloadEngine` 提供。
- 让同一个 `*permissionredis.Store` 同时作为 concrete 和 `permissionapplication.PolicyVersionPublisher` 提供。
- 让同一个 `*permissionredis.VersionTracker` 同时作为 concrete 和 `permissionapplication.PolicyVersionTracker` 提供。
- 让同一个 `*permissionredis.Watcher` 同时作为 concrete 和 `permissionredis.WatcherStatus` 提供，继续保留显式 watcher 实例化和 lifecycle 启停语义。
- 删除上述五个 concrete-to-interface helper，确保每个 constructor 只执行一次，不因多次注册同一 constructor 产生重复 Engine、Store、Tracker 或 Watcher 实例。
- 将 `NewPermissionLookup` 改为直接接收 `permissionapplication.PermissionStore`，删除 `PermissionLookupParams` 和对应 Fx import。
- 将 `NewRouteCatalogScanner` 改为直接接收 `*gin.Engine`，删除 `RouteCatalogScannerParams` 和对应 Fx import。
- 更新直接构造测试和 feature module 测试；module 测试必须验证 concrete 与各 interface 投影指向同一实例，并验证正式 graph 能启动和停止。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`：强化 RBAC composition graph 约束，要求同一 provider 同时被 concrete 与多个 interface 消费时必须提供同一实例的全部必需视图；无 DI metadata 或转换职责的构造器应直接注册或使用普通参数；结构调整不得改变 RBAC 行为。

## Impact

- 影响代码：`user-service/internal/features/permission/fx.go`、`user-service/internal/features/permission/infrastructure/casbin`、`user-service/internal/features/permission/infrastructure/redis`、`user-service/internal/features/permission/transport/http`、`user-service/internal/features/role/infrastructure/postgres` 及相关 module/direct constructor 测试。
- 影响规格：仅修改现有 `rbac-access-control` spec delta，表达长期 composition graph 约束；不把一次性 helper 删除列表写成长期 capability。
- 不改变 HTTP API、OpenAPI、Ent schema、Atlas migration、配置、部署资产、metrics 名称或日志字段。
- 不改变权限目录、route diff、角色、角色权限、用户角色、Casbin reload、Redis policy version、Pub/Sub、watcher 补偿或授权结果。
- 不创建通用 reflection helper、全局 DI facade、无业务意义的大接口或兼容 wrapper。
- 不把 user-service RBAC composition 逻辑移动到 `common` 或 `internal/shared`。
