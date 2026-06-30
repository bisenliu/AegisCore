## ADDED Requirements

### Requirement: RBAC 查询索引支撑

系统 MUST 为 RBAC 角色、权限、用户角色绑定、角色权限绑定和授权策略加载维护与稳定访问路径匹配的数据库索引，并通过 Ent schema 和 Atlas migration 交付可审查的结构变更。

#### Scenario: 角色列表和授权回源索引

- **WHEN** 系统分页查询角色、按启用状态过滤角色或在授权热路径回源查询用户启用角色
- **THEN** 角色表 MUST 提供支持过滤字段和 `role_id` 稳定排序的索引

#### Scenario: 权限列表索引

- **WHEN** 系统分页查询权限并按模块、HTTP 方法、启用状态或系统权限标记过滤
- **THEN** 权限表 MUST 提供支持常用过滤字段和 `permission_id` keyset 排序的索引

#### Scenario: 用户角色绑定反向索引

- **WHEN** 系统从角色侧 join 或反查用户角色绑定
- **THEN** 用户角色绑定表 MUST 提供以 `role_id` 起始并包含 `user_id` 的索引

#### Scenario: 角色权限绑定反向索引

- **WHEN** 系统从权限侧 join 或反查角色权限绑定
- **THEN** 角色权限绑定表 MUST 提供以 `permission_id` 起始并包含 `role_id` 的索引

#### Scenario: RBAC 索引不改变授权语义

- **WHEN** RBAC 查询索引发生调整
- **THEN** 权限目录、角色绑定、用户角色绑定、有效权限聚合、Casbin policy loader、policy sync 和超级管理员通配授权的业务结果 MUST 保持不变
