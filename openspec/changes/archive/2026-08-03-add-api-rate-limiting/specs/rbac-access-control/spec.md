## ADDED Requirements

### Requirement: RBAC 业务接口限流门禁

系统 MUST 对受 RBAC 保护的权限、角色和用户业务接口执行认证后 User ID 限流。该限流 MUST 位于认证 middleware 之后、RBAC 授权 middleware 之前，并保持授权 fail-closed 语义不变。

#### Scenario: 限流发生在授权前

- **WHEN** 已认证请求访问权限、角色或用户业务接口且对应 User ID 已超出限流阈值
- **THEN** 系统 MUST 在调用 RBAC authorizer 前拒绝请求
- **AND** 响应 MUST 为 `429 Too Many Requests`、限流错误 code 和 `success=false`

#### Scenario: 未超限请求继续授权

- **WHEN** 已认证请求未超过 User ID 限流阈值并访问受 RBAC 保护接口
- **THEN** 系统 MUST 继续使用当前用户 ID、Gin route template 和 HTTP method 执行 RBAC 授权
- **AND** 授权失败、policy 不可用或用户角色回源失败 MUST 继续返回现有 fail-closed 授权错误，不得被限流逻辑吞掉
