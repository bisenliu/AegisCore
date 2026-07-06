## ADDED Requirements

### Requirement: common Casbin wrapper 测试断言迁移

`common/security/casbin` 的测试 MUST 使用统一断言规范验证共享 Casbin authorizer wrapper、请求三元组、允许、拒绝、未配置和底层错误路径。断言迁移 MUST 保持 Casbin 三元组授权、`ErrNotConfigured`、`ErrDenied`、返回 bool/error 的 `Enforce` 语义和 error-only `Authorizer.Authorize` 语义不变。

#### Scenario: Casbin 允许和拒绝断言

- **WHEN** `common/security/casbin` 测试验证允许访问、策略拒绝或底层 enforcer 返回错误
- **THEN** 测试 MUST 使用 `require` 表达 bool 结果、错误存在性、错误类型和错误包装断言
- **AND** 迁移 MUST NOT 改变 `Enforce` 或 `Authorizer.Authorize` 的授权结果语义

#### Scenario: 未配置 authorizer 断言

- **WHEN** 测试验证 nil enforcer、未配置 authorizer 或非法请求三元组路径
- **THEN** 测试 MUST 使用语义化断言表达 `ErrNotConfigured`、`ErrDenied` 或参数校验结果
- **AND** 迁移 MUST NOT 将未配置、拒绝访问或底层错误折叠为无法区分的测试结果

#### Scenario: 不影响 user-service RBAC

- **WHEN** common Casbin wrapper 测试迁移断言风格
- **THEN** user-service 的权限目录、角色绑定、用户角色绑定、policy loader、policy sync、超级管理员通配授权和 RBAC HTTP 授权行为 MUST 保持不变
- **AND** 迁移 MUST NOT 修改 user-service feature 测试或 RBAC 生产代码
