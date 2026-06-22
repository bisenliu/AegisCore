## MODIFIED Requirements

### Requirement: Casbin 授权保护

系统 MUST 使用 RBAC 授权中间件保护权限、角色和用户业务接口，并在认证通过后执行资源级授权判断。Casbin subject/object/action MUST 分别使用 `user:<user_uuid>`、`role:<role_uuid>`、Gin route template 和 HTTP method。

#### Scenario: 授权通过

- **WHEN** 已认证用户拥有当前 HTTP 方法和路径对应权限
- **THEN** 系统 MUST 允许请求进入目标 controller

#### Scenario: 授权失败

- **WHEN** 已认证用户缺少当前 HTTP 方法和路径对应权限
- **THEN** 系统 MUST 拒绝请求并返回授权失败错误

#### Scenario: 权限策略更新

- **WHEN** 权限、角色或绑定发生变化
- **THEN** 系统 MUST 同步或刷新授权策略，避免旧策略长期影响授权判断

#### Scenario: Casbin policy 权威来源

- **WHEN** policy loader 从持久化层构造授权策略
- **THEN** 策略 MUST 由启用角色、启用权限、角色权限绑定和用户角色绑定派生，不得以独立 `casbin_rules` 表作为业务权威来源

#### Scenario: Casbin subject 稳定格式

- **WHEN** 角色参与 policy 构造或授权判断
- **THEN** 角色 subject MUST 使用 `role:<role_uuid>`，不得依赖 `roles.code`；用户身份解析 MUST 排除已软删除用户

#### Scenario: 超级管理员通配授权

- **WHEN** 用户拥有 `internal/shared/rbacbaseline` 中稳定的内置超级管理员角色
- **THEN** policy loader MUST 补充 wildcard policy，使其可访问受保护业务接口，且 MUST NOT 在 role 或 permission feature 内重复定义超级管理员常量
