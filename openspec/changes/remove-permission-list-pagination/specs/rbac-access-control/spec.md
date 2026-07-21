## MODIFIED Requirements

### Requirement: 权限目录与路由诊断

系统 MUST 将 `internal/shared/rbacbaseline.DefaultPermissions()` 作为权限定义的唯一业务权威来源，并将 permissions 数据库表作为供列表、角色绑定和授权加载使用的只读投影。权限 MUST 使用稳定 `permission_id` 描述可授权的 HTTP method、route template 和业务模块；运行时 MUST NOT 提供权限创建、详情、更新、启停或 route diff 公开接口。

#### Scenario: 权限查询边界

- **WHEN** user-service 注册权限 HTTP 路由
- **THEN** 系统 MUST 只注册 `GET /api/v1/permissions` 和 `GET /api/v1/permissions/users/:user_id/effective`
- **AND** 系统 MUST NOT 注册权限创建、详情、更新、启用、停用或 route diff HTTP 路由
- **WHEN** 授权调用方查询权限目录
- **THEN** 系统 MUST 按稳定权限 ID 排序返回完整匹配权限投影集合
- **AND** 权限列表请求 MUST 只支持 `module` 和 `http_method` 过滤参数，MUST NOT 接受或展示 `cursor` 或 `page_size` 分页参数
- **AND** 权限列表成功响应 MUST 使用 `data.items` 包装权限集合，MUST NOT 包含 `data.pagination`
- **AND** 列表输入和响应 MUST NOT 包含 `active`、`is_system` 或 `system`
- **WHEN** 授权调用方使用非法 `http_method` 查询权限目录
- **THEN** 系统 MUST 返回 `400 Bad Request`

#### Scenario: 权限定义和 seed 投影

- **WHEN** 运维执行 RBAC seed
- **THEN** 系统 MUST 按 `rbacbaseline.DefaultPermissions()` 中的稳定 `permission_id` 幂等 upsert 权限名称、描述、模块、HTTP method 和 route template
- **AND** method 或 route template 修改 MUST 沿用原 `permission_id`，使已有角色权限绑定保持不变
- **AND** 权限实体、seed 输入和数据库投影 MUST NOT 包含权限启停或系统权限标记

#### Scenario: 受控删除权限

- **WHEN** 权限从 `rbacbaseline.DefaultPermissions()` 删除
- **THEN** 受控 migration MUST 先删除对应 `role_permissions` 再删除 `permissions` 记录
- **AND** seed 和 HTTP 运行时 MUST NOT 自动删除基线之外的权限或绑定
- **AND** 数据清理后系统 MUST 通过显式 policy reload 或滚动重启使 Casbin policy 收敛

#### Scenario: 路由与权限基线一致性门禁

- **WHEN** CI 或测试构建真实 Gin route graph 并扫描 `/api/v1` 下需要 RBAC 授权的路由
- **THEN** 系统 MUST 将 HTTP method 和 route template 与 `rbacbaseline.DefaultPermissions()` 双向比较
- **AND** 任一实际路由缺少基线权限或任一基线权限没有对应实际路由时测试 MUST 失败
- **AND** 扫描 MUST 排除 `OPTIONS`、认证公开接口和会话控制接口
- **AND** 一致性校验 MUST NOT 创建或修改权限、角色绑定或运行时 policy
