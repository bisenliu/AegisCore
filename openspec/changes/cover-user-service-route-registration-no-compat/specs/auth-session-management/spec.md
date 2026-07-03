## ADDED Requirements

### Requirement: 认证路由注册测试覆盖
系统 MUST 使用 router 包测试覆盖认证公开路由和认证保护路由在 user-service 聚合路由中的注册结果，确保认证入口仅存在于当前 `/api/v1/auth` 路由图中。

#### Scenario: 认证公开路由注册
- **WHEN** `registerV1Routes` 注册当前 `/api/v1` 路由组
- **THEN** 测试 MUST 验证登录、refresh 和强制改密入口注册在 `/api/v1/auth` 下
- **AND** 测试 MUST 验证这些公开认证路由不经过普通 access token 认证中间件

#### Scenario: 认证保护路由注册
- **WHEN** `registerV1Routes` 注册当前 `/api/v1` 路由组
- **THEN** 测试 MUST 验证退出当前会话和退出全部会话入口注册在 `/api/v1/auth` 下
- **AND** 测试 MUST 验证这些路由进入当前认证中间件链
- **AND** 测试 MUST NOT 为旧认证绕过路径或 `/api`、`/v1` 旧别名保留兼容断言
