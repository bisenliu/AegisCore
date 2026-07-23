## Why

permission feature 当前在 Fx composition 中通过多个 `providePermissionAuthorizer`、`providePermissionPolicyHealth`、`providePermissionPolicyWatcherStatus` 等函数，把 named/private RBAC runtime 组件逐个转发为 public 依赖。该模式增加了 provider 样板、生命周期依赖分散和测试断言成本，但并未表达这些组件属于同一组 permission runtime 的事实。

## What Changes

- 在 permission feature 内引入 `PermissionRuntime` 聚合对象，集中承载授权、policy health、watcher 状态、policy change notifier、policy initializer、watcher runner 和 user role resolver lifecycle。
- 将 named/private RBAC runtime 组件的 public 投影收敛为从 `PermissionRuntime` 解包或直接消费聚合对象，减少重复 provider 函数和 Fx graph 样板。
- 调整 permission、role、providers 中使用授权、健康检查、watcher 状态和 lifecycle hook 的消费方，使其依赖关系保持清晰且不改变运行时行为。
- 更新相关 Fx 测试，覆盖 runtime 聚合对象组装和对外依赖仍可解析。
- 不改变 HTTP API、Casbin 策略语义、Redis policy sync 协议、数据库 schema、OpenAPI 或部署资产。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 收敛 RBAC runtime 组件在 Fx graph 中的组装和公开投影方式；不改变外部行为要求。

## Impact

- 主要影响 `user-service/internal/features/permission/fx_authorization.go`、`user-service/internal/features/permission/fx_sync.go`、`user-service/internal/features/permission/fx_lifecycle.go` 和 `user-service/internal/features/permission/fx_test.go`。
- 可能影响依赖 permission public projection 的消费方：`user-service/internal/providers/routes.go`、`user-service/internal/providers/health.go`、`user-service/internal/features/role/fx.go`。
- 不影响 HTTP 路由、响应契约、OpenAPI、Ent schema、Atlas migration、Redis key/schema、Casbin model 或部署配置。
