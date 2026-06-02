## ADDED Requirements

### Requirement: Centralize shared authentication response message
系统 SHALL 在 `common` 的响应契约中集中维护通用认证失败公开文案，使认证中间件、服务调用方和测试能够复用同一 message 常量。

#### Scenario: Authentication failure message is reusable
- **WHEN** common 认证中间件构造未认证、token 非法或 token 过期失败响应
- **THEN** 响应 message MUST 来自 `common` 的共享认证失败 message 常量
- **THEN** message 值 MUST 保持为 `登录状态无效或已过期，请重新登录`

#### Scenario: Response codes remain stable
- **WHEN** 共享认证失败 message 常量被用于失败响应
- **THEN** 未认证响应业务码 MUST 保持为 `20000`
- **THEN** token invalid 响应业务码 MUST 保持为 `20001`
- **THEN** token expired 响应业务码 MUST 保持为 `20002`
- **THEN** 响应 envelope 结构 MUST 保持不变

#### Scenario: Internal error and success messages remain centralized
- **WHEN** 实现迁移或整合 response message 常量
- **THEN** `ok`、`created` 和 `internal server error` 的常量来源 MUST 继续位于 `common/response`
- **THEN** 成功响应和内部错误响应的 message 值 MUST 保持不变
