## ADDED Requirements

### Requirement: 认证入口请求体容量边界

系统 MUST 对 `/api/v1/auth` 下需要 JSON 请求体的认证入口执行请求体字节上限检查。超限请求 MUST 在密码校验、refresh token 解析、password-change session 消费或会话撤销 use case 前被拒绝，并 MUST 返回 `413 Payload Too Large` 与统一错误 envelope。

#### Scenario: 公开认证入口拒绝超限请求体

- **WHEN** 调用方向登录、refresh 或强制改密入口提交超过配置上限的 JSON 请求体
- **THEN** 系统 MUST 返回 `413 Payload Too Large`
- **AND** 系统 MUST NOT 执行密码哈希校验、refresh token 解析、password-change session 消费、token 签发或 session 创建

#### Scenario: 认证入口覆盖固定长度和 chunked 载荷

- **WHEN** 超限认证请求体使用固定 `Content-Length` 或 chunked 传输
- **THEN** 两类请求 MUST 都被同一容量边界拒绝
- **AND** 认证错误、限流错误和字段校验错误语义 MUST 保持与非超限请求一致，不得互相伪装

#### Scenario: 认证入口尾随 JSON 超限

- **WHEN** 认证请求体首个 JSON 文档合法，但其后追加的尾随 JSON 使总请求体超过配置上限
- **THEN** 系统 MUST 返回 `413 Payload Too Large`
- **AND** auth use case MUST NOT 被调用
