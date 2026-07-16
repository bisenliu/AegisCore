## 1. 审计与边界确认

- [x] 1.1 审计 `user-service/internal/features/user`、`auth`、`role`、`permission` composition 中所有 identity projection、constructor forwarding 和单字段 Fx Params，记录保留与删除原因。
- [x] 1.2 确认本 change 只修改 RBAC permission/role composition 相关实现，不并行处理 auth command use case 的重复 Params、历史 optional metrics 或 credential verifier 输入适配。
- [x] 1.3 确认具名 Ent、Redis、worker pool、cache 依赖使用的 Fx Params/tag、`cmd.newBootstrapLifecycleApp` 测试 seam、application port 所有权和 RBAC policy sync 模型均保持不变。

## 2. Permission Composition 实现

- [x] 2.1 修改 `user-service/internal/features/permission/fx.go`，直接注册 `commonmetrics.NewCasbinPolicyReloadMetrics`，删除无语义 metrics 转发函数。
- [x] 2.2 直接注册 `permissionauthorization.NewAuthorizer`，删除对相同返回类型的冗余 `fx.As(new(permissionauthorization.Authorizer))`。
- [x] 2.3 使用 `fx.Annotate`、`fx.As` 和 `fx.Self` 注册 `permissioncasbin.NewEngine`，让同一个 `*permissioncasbin.Engine` 同时提供 concrete、`permissionauthorization.Engine` 和 `permissionapplication.PolicyReloadEngine`。
- [x] 2.4 使用 `fx.Annotate`、`fx.As` 和 `fx.Self` 注册 Redis Store、VersionTracker 和 Watcher，让同一实例同时提供 concrete 与对应 interface 视图。
- [x] 2.5 删除 `newAuthorizationEngine`、`newPolicyReloadEngine`、`newPolicyVersionPublisher`、`newPolicyVersionTracker` 和 `newPolicyWatcherStatus` 五个 concrete-to-interface helper，并确认每个 constructor 只注册一次。
- [x] 2.6 将 `permissioncasbin.Params.Metrics` 改为必需输入，删除 `optional:"true"`；metrics 禁用时继续通过 `commonmetrics.NewCasbinPolicyReloadMetrics` 注入 `NopReloadMetrics()`。

## 3. 普通构造器参数清理

- [x] 3.1 将 `role/infrastructure/postgres.NewPermissionLookup` 改为直接接收 `permissionapplication.PermissionStore`，删除 `PermissionLookupParams` 和对应 Fx import。
- [x] 3.2 将 `permission/transport/http.NewRouteCatalogScanner` 改为直接接收 `*gin.Engine`，删除 `RouteCatalogScannerParams` 和对应 Fx import。
- [x] 3.3 更新相关直接构造调用点，确保没有新增兼容 wrapper 或无业务意义大接口。

## 4. 测试更新

- [x] 4.1 更新 Casbin Engine 直接构造测试；不观察 reload metrics 的测试显式传入 `commonmetrics.NopReloadMetrics()`。
- [x] 4.2 增加或更新正式 permission module 测试，验证缺少 reload metrics provider 时 graph 构造失败，证明该输入边不是 optional。
- [x] 4.3 更新 PermissionLookup 和 RouteCatalogScanner 的直接单元测试，使用普通参数构造，不再依赖单字段 Fx Params。
- [x] 4.4 扩展 permission/role module graph 测试，验证 `*permissioncasbin.Engine` concrete 与 `permissionauthorization.Engine`、`permissionapplication.PolicyReloadEngine` 指向同一实例。
- [x] 4.5 扩展 permission/role module graph 测试，验证 `*permissionredis.Store`、`*permissionredis.VersionTracker`、`*permissionredis.Watcher` concrete 与各 interface 投影指向同一实例。
- [x] 4.6 扩展 module graph 测试，验证 watcher 被强制实例化，正式 graph 能启动和停止，初始 load 以及 health/metrics consumer 所需 concrete/interface 均可解析。

## 5. 验证与收尾

- [x] 5.1 运行 `go test -count=1 ./internal/features/permission/... ./internal/features/role/...`。
- [x] 5.2 运行 `make user-service-architecture-lint`。
- [x] 5.3 运行 `openspec validate standardize-rbac-fx-composition`。
- [x] 5.4 检查 API、OpenAPI、Ent schema、Atlas migration、配置、部署资产、metrics 名称和日志字段没有非预期变更或生成物 drift。
- [x] 5.5 暂存本 change 的预期代码、测试、规格和文档变更。
- [x] 5.6 在暂存预期变更后运行 `make lint`。
- [x] 5.7 在暂存预期变更后运行 `make verify`。
- [x] 5.8 检查最终 diff，确认没有生成物 drift、无关文件变更或超出本 change 范围的修改。
