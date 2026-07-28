## ADDED Requirements

### Requirement: 认证入口限流门禁

系统 MUST 对 `/api/v1/auth` 下不需要普通 access token 的认证入口执行匿名 IP 限流，对需要普通 access token 的会话控制入口执行 User ID 限流。限流 MUST 不改变 token 签发、refresh、强制改密、退出或 token version 校验的安全语义。

#### Scenario: 公开认证入口先限流

- **WHEN** 调用方访问登录、refresh 或强制改密入口
- **THEN** 系统 MUST 在 auth controller 执行前按客户端 IP 执行匿名限流
- **AND** 超限时 MUST 返回 `429 Too Many Requests`，MUST NOT 执行密码校验、refresh token 解析或 password-change session 消费

#### Scenario: 已认证会话控制按 User ID 限流

- **WHEN** 调用方访问退出当前会话或退出全部会话接口
- **THEN** 系统 MUST 先校验 bearer access token 和 token version
- **AND** 校验通过后 MUST 按认证 User ID 执行限流，再进入 auth controller

#### Scenario: 限流不改变认证错误

- **WHEN** bearer token 缺失、格式非法、过期、签名无效或 token version 不匹配
- **THEN** 系统 MUST 返回现有认证错误响应
- **AND** 系统 MUST NOT 将认证失败映射为限流错误
