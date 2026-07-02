## Why

`role/application/command` 的 command service 测试需要用同一种方式表达角色写操作、用户角色绑定、角色权限绑定和 RBAC policy 变更通知的依赖契约。将手写 store/notifier double 收敛到已有 `mockgen` 生成物，可以让调用参数、调用次数、失败路径和通知吞错行为通过 expectation 直接暴露，降低后续 RBAC 行为演进时测试替身与真实 port drift 的风险。

## What Changes

- 将 `user-service/internal/features/role/application/command` 包内 role、user-role、role-permission、permission lookup 和 policy change notifier 测试协作者统一使用已有 gomock 生成物。
- 不保留 `roleTestStore`、`userRoleTestStore`、`rolePermissionTestStore`、`permissionLookupTestStore`、`recordingRolePolicyChangeNotifier` 等手写兼容测试替身。
- 用 gomock expectation、`gomock.InOrder`、matcher 或 `DoAndReturn` 明确表达系统角色保护、权限查找、用户角色绑定、角色权限绑定、输入去重和 policy change 通知失败吞掉等断言。
- 保留只负责构造输入、领域对象或 matcher 的测试 helper，不改变生产 command service、domain 规则、permission notifier 实现或 Postgres adapter。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 明确 role command service 测试应使用已有 gomock 生成物表达 role、user-role、role-permission、permission lookup 和 policy change notifier 依赖契约；生产 RBAC 行为不变。

## Impact

- 影响代码范围限定在 `user-service/internal/features/role/application/command` 包测试和该包已有 `mock_generate.go` 覆盖的生成 mock。
- 不影响 role command 生产代码、permission application notifier、PostgreSQL adapter、HTTP API、OpenAPI、数据库 schema、Atlas migration、Redis policy sync 语义、部署资产或共享契约。
- 验证要求包括 `make user-service-generate` 后无 mockgen drift、`cd user-service && go test ./internal/features/role/application/command` 通过，以及 `make user-service-architecture-lint` 通过。
