## ADDED Requirements

### Requirement: Use explicit user creation handler names

用户资料创建能力 SHALL 在 controller 和路由注册中使用能独立表达用户创建语义的 handler 名称。实现 MUST 保持 `POST /api/v1/users` 的请求体校验、认证要求、响应信封、HTTP 201 成功语义、冲突错误语义和分层职责不变。

#### Scenario: User creation route uses explicit handler name
- **Given** 用户创建路由已注册
- **When** 开发者检查 `POST /api/v1/users` 的 Gin handler 引用
- **Then** 路由 MUST 引用 `UserController.CreateUser`
- **Then** 路由 MUST NOT 引用 `UserController.Create`

#### Scenario: User creation API behavior remains unchanged
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `POST /api/v1/users` 并提交合法 JSON 请求体
- **Then** 系统 MUST 继续创建用户并返回 HTTP 201
- **Then** 请求绑定、输入清洗、service 编排和 repository 持久化职责 MUST 保持不变
