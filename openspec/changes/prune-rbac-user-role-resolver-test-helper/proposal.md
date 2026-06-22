## Why

`policy.go` 中的 `newUserRoleResolver` 只被同包测试使用，生产代码已经直接在 `NewUserRoleResolver` 中构造 `entUserRoleResolver`。保留这个仅服务测试的 helper 会让生产文件暴露不必要的内部构造路径，也弱化了测试对真实结构的直接覆盖。

## What Changes

- 删除 `user-service/internal/features/permission/infrastructure/casbin/policy.go` 中仅被测试使用的 `newUserRoleResolver` helper。
- 调整 `user-service/internal/features/permission/infrastructure/casbin/policy_test.go`，让测试直接构造 `&entUserRoleResolver{cache: cache}`。
- 保持 `NewUserRoleResolver` 的 Fx 构造逻辑、缓存配置、失效方法、角色查询行为和现有测试断言不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 整理 permission casbin 用户角色 resolver 测试辅助构造，保持 RBAC 用户角色解析缓存、失效和授权热路径需求语义不变。

## Impact

- 受影响代码：
  - `user-service/internal/features/permission/infrastructure/casbin/policy.go`
  - `user-service/internal/features/permission/infrastructure/casbin/policy_test.go`
- API、数据库 schema、OpenAPI、部署、观测、安全语义和共享契约无变化。
- 验证重点是 permission casbin 包测试，确保用户角色缓存命中、失效和并发 miss 合并行为保持不变。
