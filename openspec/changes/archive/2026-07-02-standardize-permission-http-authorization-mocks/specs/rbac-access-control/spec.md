## ADDED Requirements

### Requirement: Permission HTTP 授权中间件请求构造与旁路行为
permission HTTP 授权中间件 MUST 在真实 Gin 路由上下文中解析授权请求，并将认证用户 ID、Gin route template 和 HTTP method 传递给 permission authorization service；白名单和 `OPTIONS` 请求 MUST 绕过授权服务调用。

#### Scenario: 授权请求使用 Gin route template
- **WHEN** 已认证用户访问受 RBAC 保护的 permission HTTP 路由
- **THEN** 授权中间件 MUST 使用认证用户 ID 作为授权 subject
- **AND** 授权中间件 MUST 使用 Gin `FullPath()` route template 作为授权 object
- **AND** 授权中间件 MUST 使用 HTTP method 作为授权 action

#### Scenario: 认证用户来自请求上下文
- **WHEN** 已认证用户 ID 存在于 request context 且 Gin context 未设置用户 ID
- **THEN** 授权中间件 MUST 使用 request context 中的用户 ID 构造授权请求

#### Scenario: 缺失或非法用户不调用授权服务
- **WHEN** 请求缺少认证用户 ID 或 Gin context 中的用户 ID 类型非法
- **THEN** 授权中间件 MUST 拒绝请求并返回未认证错误
- **AND** 授权中间件 MUST NOT 调用 permission authorization service

#### Scenario: 白名单请求绕过授权服务
- **WHEN** 请求方法和 Gin route template 命中显式授权白名单
- **THEN** 授权中间件 MUST 允许请求继续处理
- **AND** 授权中间件 MUST NOT 调用 permission authorization service

#### Scenario: OPTIONS 请求绕过授权服务
- **WHEN** 请求使用 `OPTIONS` 方法访问已注册路由
- **THEN** 授权中间件 MUST 允许请求继续处理
- **AND** 授权中间件 MUST NOT 调用 permission authorization service

#### Scenario: 授权服务拒绝或错误映射响应
- **WHEN** permission authorization service 返回拒绝、执行错误或 invalid subject 错误
- **THEN** 授权中间件 MUST 分别返回禁止访问、内部错误或未认证错误响应
