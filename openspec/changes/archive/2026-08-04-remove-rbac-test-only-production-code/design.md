## Context

RBAC 当前在三个边界保留生产不可达内容：permission Redis watcher 的 `newWatcherWithMetrics` 只被测试 fake 调用；`rbacbaseline.permissionIDs` 只被自身测试和注释示例引用；permission HTTP transport 的 `WithAuthorizationWhitelist` 只转发 common middleware helper，生产路由没有消费者。`defaultRoleCatalog` 同时包含未来默认角色的整块注释模板。

生产调用图 deadcode 检查会报告这三个函数，而 `-test` 调用图会因测试引用隐藏它们。Ent schema/mixin 的生成期入口是工具无法识别的合理例外，不属于本次清理。另有 `internal/config.LoadFromDocuments` 测试支持入口，不属于 RBAC capability 和本 change 范围。

受影响路径仅包括 `user-service/internal/features/permission/infrastructure/redis/`、`user-service/internal/features/permission/transport/http/`、`user-service/internal/shared/rbacbaseline/` 及 OpenSpec artifacts。`common/` 的通用 Casbin middleware、role feature、deployments、OpenAPI 和观测资产保持不变。

## Goals / Non-Goals

**Goals:**

- 生产 RBAC 源码不再包含只为测试存在的构造器或 helper。
- 测试继续覆盖 watcher fake store、显式授权白名单和 RBAC 基线一致性。
- 删除 feature-local alias/wrapper，直接消费 common middleware 的稳定类型和 option。
- 保持 watcher、默认角色绑定、授权白名单和 fail-closed 的运行时行为不变。

**Non-Goals:**

- 不删除或修改 `common/http/middleware` 的公开 Casbin API。
- 不新增默认角色、白名单路由、配置开关、兼容 alias、回退分支或测试生产 hook。
- 不处理 Ent 生成期 deadcode 报告或 `internal/config` 测试支持入口。
- 不修改 Ent schema、Atlas migration、OpenAPI、部署、观测或外部 API。

## Decisions

### Decision: watcher fake 构造只存在于测试编译单元

删除生产文件中的 `newWatcherWithMetrics`。需要 `policySubscriptionStore` fake 的测试在 `_test.go` 定义 `newWatcherForTest`，并调用生产内部核心构造 `newWatcher`；能提供真实 `*Store` 的测试继续直接调用 `NewWatcher`。这样生产仍只有 `NewWatcher` 公开构造入口，测试无需扩大 `WatcherParams.Store` 类型。

不把 `WatcherParams.Store` 从 `*Store` 改为接口，因为这会为了测试扩大生产 API；也不保留旧 helper 转发，因为它正是本次要删除的生产不可达入口。

### Decision: baseline 只表达当前真实角色

删除 `permissionIDs`、其 helper 自测和未来默认角色注释模板。保留当前被生产消费的 `defaultRoleCatalog`、`allPermissionIDs` 以及公开基线一致性测试。未来真实新增默认角色时，在对应 OpenSpec change 中随实际 catalog block 引入所需实现，不提前保留 helper 或伪代码。

不迁移注释到其他生产文件，也不保留空扩展分支；未来角色的权限显式列举约束继续由主规格拥有。

### Decision: permission transport 直接使用 common authorization option

删除 `AuthorizationWhitelistRule`、`AuthorizationOption` alias 和 `WithAuthorizationWhitelist` wrapper。`Authorize` 的可选参数直接声明为 `commonmiddleware.CasbinAuthorizationOption`，permission transport 测试直接调用 `WithCasbinAuthorizationWhitelist` 和 `CasbinAuthorizationWhitelistRule`，继续证明 feature middleware 正确转发白名单行为。

不删除 `Authorize` 的 option 参数，因为主规格要求显式白名单语义；不在 router 装配虚构白名单消费者，也不保留 feature-local 名称兼容层。

## Risks / Trade-offs

- [Risk] watcher 测试默认节奏与生产默认值漂移 → Mitigation：测试 helper 只设置测试需要覆盖的 `CheckInterval`，其余值仍由生产 `WatcherSettings.applyDefaults` 统一处理。
- [Risk] 删除 feature-local alias 后测试 import 更长 → Mitigation：接受显式 common 类型，换取唯一能力来源和更清晰的所有权。
- [Risk] deadcode 输出仍包含生成期和其他测试支持入口 → Mitigation：验证时精确断言本 change 的三个符号消失，不把工具误报当作零输出目标。

## Migration Plan

1. 先提交并校验本 change artifacts，再按 watcher、baseline、HTTP authorization 的顺序实施清理。
2. 对修改的 Go 文件运行 `gofmt`，执行相关 user-service 与 common middleware 测试以及生产调用图 deadcode 检查。
3. 运行架构 lint；暂存本次全部预期变更后运行 `make lint` 和 `make verify`。
4. 不存在数据或发布迁移，也不保留兼容回滚路径；若验证失败，直接修正当前单一路径后重新验证。

## Open Questions

无。
