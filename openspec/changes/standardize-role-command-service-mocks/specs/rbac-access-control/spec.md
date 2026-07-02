## ADDED Requirements

### Requirement: role command service 测试协作者契约

role command service 测试 MUST 使用 `user-service/internal/features/role/application/command` 包已有 `mockgen` 生成物表达 `RoleStore`、`UserRoleStore`、`RolePermissionStore`、`PermissionLookup` 和 `PolicyChangeNotifier` 等外部协作者契约。测试 MUST NOT 保留实现这些 port 的手写 store/notifier double 来兼容或隐藏依赖调用、失败路径、调用顺序、去重逻辑或 policy change 通知行为。

#### Scenario: 角色生命周期测试使用生成 mock

- **WHEN** command 包测试覆盖角色创建、更新、启用、停用、重复角色或系统角色保护路径
- **THEN** 测试 MUST 通过生成 mock 的 expectation 表达 `RoleStore` 调用、输入归一化、错误映射和禁止写入路径
- **AND** 系统角色保护相关测试 MUST 明确断言受保护变更不会调用后续写入或 policy change 通知

#### Scenario: 用户角色绑定测试使用生成 mock

- **WHEN** command 包测试覆盖用户角色添加、移除、替换、角色不存在或重复角色 ID 去重路径
- **THEN** 测试 MUST 通过生成 mock 的 expectation、matcher 或 `DoAndReturn` 表达 `RoleStore` 查询、`UserRoleStore` 写入和返回角色集合
- **AND** 用户角色绑定成功后的用户角色缓存失效通知 MUST 通过 `PolicyChangeNotifier` expectation 明确断言

#### Scenario: 角色权限绑定测试使用生成 mock

- **WHEN** command 包测试覆盖角色权限添加、移除、替换、权限不存在、权限不可用或重复权限 ID 去重路径
- **THEN** 测试 MUST 通过生成 mock 的 expectation 表达 `PermissionLookup` 校验和 `RolePermissionStore` 写入
- **AND** 权限查找失败或权限不可用时，测试 MUST 明确断言不会执行角色权限写入或 policy change 通知

#### Scenario: policy change 通知失败被明确覆盖

- **WHEN** 角色写操作、用户角色绑定或角色权限绑定已经成功，但 `PolicyChangeNotifier.NotifyPolicyChanged` 返回错误
- **THEN** 测试 MUST 通过生成 mock expectation 注入通知错误
- **AND** 测试 MUST 断言 command service 按既有语义吞掉通知失败并返回写操作成功结果

#### Scenario: 保留纯测试 helper

- **WHEN** command 包测试需要复用角色、权限引用、输入命令、gomock matcher 或 service fixture 构造逻辑
- **THEN** 保留的 helper MUST NOT 实现 `RoleStore`、`UserRoleStore`、`RolePermissionStore`、`PermissionLookup` 或 `PolicyChangeNotifier` port
- **AND** 这些 helper MUST NOT 替代生成 mock 记录 collaborator 调用或隐藏失败注入
