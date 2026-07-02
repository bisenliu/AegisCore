## ADDED Requirements

### Requirement: RBAC seed service 测试协作者契约

RBAC seed service 测试 MUST 使用 `user-service/internal/features/role/application/seed` 包已有 `mockgen` 生成物表达 `SeedRoleStore`、`SeedPermissionStore`、`SeedRolePermissionStore` 和 `SeedUserRoleStore` 等外部持久化协作者契约。测试 MUST NOT 保留实现这些 seed port 的手写 store double 来兼容或隐藏依赖调用、失败路径、调用顺序、重复 seed、reactivate 参数、binding 同步或超级管理员绑定行为。

#### Scenario: 默认 seed 测试使用生成 mock

- **WHEN** seed 包测试覆盖默认系统角色、系统权限和系统角色权限绑定初始化路径
- **THEN** 测试 MUST 通过生成 mock 的 ordered expectation 表达 `SeedRoleStore.UpsertSystemRole`、`SeedPermissionStore.UpsertSystemPermission` 和 `SeedRolePermissionStore.EnsureSystemBindings` 调用
- **AND** 测试 MUST 基于 `rbacbaseline.DefaultRoles()`、`DefaultPermissions()` 和 `DefaultRolePermissions()` 校验调用数量、参数映射和返回统计

#### Scenario: 重复 seed 测试使用生成 mock

- **WHEN** seed 包测试覆盖默认系统数据已经存在的重复 seed 路径
- **THEN** 测试 MUST 通过生成 mock 返回已存在写入结果，并断言 `SeedResult` 的 inserted、updated 和 binding added 统计保持既有语义
- **AND** 测试 MUST NOT 依赖手写 store double 的内部状态模拟重复数据

#### Scenario: reactivate 和 sync bindings 测试使用生成 mock

- **WHEN** seed 包测试覆盖 `ReactivateSystem` 或 `SyncSystemBindings` 选项
- **THEN** 测试 MUST 通过 matcher 明确断言角色和权限 upsert 输入携带正确的 reactivate 参数
- **AND** `SyncSystemBindings` 场景 MUST 通过 `SeedRolePermissionStore.SyncSystemBindings` expectation 表达新增和删除绑定统计

#### Scenario: assign super admin 测试使用生成 mock

- **WHEN** seed 包测试覆盖为用户分配内置超级管理员角色
- **THEN** 测试 MUST 通过 `SeedUserRoleStore.AssignRole` expectation 断言用户 ID 和 `rbacbaseline.SuperAdminRoleID` 对应角色 ID
- **AND** 测试 MUST 覆盖新增绑定和已有绑定两类返回结果

#### Scenario: 保留纯测试 helper

- **WHEN** seed 包测试需要复用 service fixture、输入 matcher、UUID 解析或 baseline 期望构造逻辑
- **THEN** 保留的 helper MUST NOT 实现 `SeedRoleStore`、`SeedPermissionStore`、`SeedRolePermissionStore` 或 `SeedUserRoleStore` port
- **AND** 这些 helper MUST NOT 替代生成 mock 记录 collaborator 调用或隐藏失败注入
