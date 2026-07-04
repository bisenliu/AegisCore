## ADDED Requirements

### Requirement: 受保护 HTTP flow 授权边界断言规范
系统 MUST 使用语义化断言覆盖 user-service E2E HTTP flow 中受保护用户接口的认证和授权边界。断言迁移 MUST 保持当前认证中间件、RBAC 授权中间件、错误 envelope 和受保护路由语义不变。

#### Scenario: 授权上下文访问用户接口
- **WHEN** E2E flow 使用当前测试前置条件中的有效 bearer token 访问用户创建或用户详情接口
- **THEN** 测试 MUST 使用语义化断言验证请求进入当前受保护 HTTP flow 并返回预期 response envelope
- **AND** 迁移 MUST NOT 绕过认证或 RBAC 中间件，也 MUST NOT 新增旧授权兼容断言

#### Scenario: 缺失认证访问受保护接口
- **WHEN** E2E flow 未提供 bearer token 访问受保护用户接口
- **THEN** 测试 MUST 使用语义化断言验证 HTTP `401 Unauthorized`、`success=false` 和 `CodeUnauthenticated`
- **AND** 测试 MUST NOT 接受旧认证绕过路径、旧错误码或旧 envelope 格式

#### Scenario: 跨 feature 响应断言保持边界
- **WHEN** E2E flow 同时经过认证、RBAC 和用户资料 feature 的响应边界
- **THEN** 测试 MUST 只迁移断言表达
- **AND** 测试 MUST NOT 修改 Casbin policy、RBAC seed、角色权限绑定、用户角色绑定、受保护路由路径或授权结果语义
