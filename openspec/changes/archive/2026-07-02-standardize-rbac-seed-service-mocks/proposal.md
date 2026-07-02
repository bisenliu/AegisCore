## Why

RBAC seed service 测试当前依赖手写 store double，容易与已生成 gomock 接口漂移，且调用顺序、调用次数和参数匹配不够显式。将测试统一到已有 gomock 生成物后，可以让默认 seed、重复 seed、reactivate、sync bindings 和 assign super admin 的持久化交互更可审查，并降低后续接口演进时的维护成本。

## What Changes

- 将 `user-service/internal/features/role/application/seed` 包内测试的手写 seed store 替换为已有 gomock 生成物。
- 移除 `seedRoleTestStore`、`seedPermissionTestStore`、`seedRolePermissionTestStore`、`seedUserRoleTestStore` 等兼容测试路径。
- 使用 ordered expectation 明确表达默认 seed、重复 seed、reactivate、sync bindings 和 assign super admin 场景的调用顺序。
- 使用循环和 matcher 覆盖 `rbacbaseline.DefaultRoles()`、`DefaultPermissions()`、`DefaultRolePermissions()` 的调用数量、参数和返回值。
- 不修改 RBAC baseline catalog、seed service 生产逻辑、RBAC CLI、Postgres seed adapter、role command/query 测试或跨 feature seed mock。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 明确 RBAC seed service 测试应使用已有 gomock 生成物表达 seed 相关持久化端口依赖契约；生产 RBAC 行为不变。

## Impact

- 影响代码范围仅限 `user-service/internal/features/role/application/seed` 包内测试。
- 复用已有 `mock_generate.go` 生成的 `SeedRoleStore`、`SeedUserRoleStore`、`SeedRolePermissionStore` mock。
- 复用已有 `mock_permission_test.go` 生成的 `SeedPermissionStore` mock。
- 不改变 HTTP API、OpenAPI、数据库 schema、migration、部署资产、观测资产或生产运行时依赖。
- 验证命令包括 `make user-service-generate`、`cd user-service && go test ./internal/features/role/application/seed`、`make user-service-architecture-lint` 和最终 `make verify`。
