## MODIFIED Requirements

### Requirement: Provide JWT token credential primitives
系统 SHALL 在 `common/security/auth` 包提供 JWT token 凭证服务，用于签发和解析访问 token、刷新 token 和密码变更 token。JWT 服务 MUST 继续使用现有认证配置中的 secret、issuer 和 audience，并 MUST 保持现有 claims、subject、过期时间、签名算法和用户身份字段校验语义。JWT 服务 MUST 区分缺失的 `user_id` 与格式非法的 `user_id`：当 `user_id` 为空时 MUST 返回缺失用户 ID 错误，当 `user_id` 存在但不是合法 UUID 时 MUST 返回无效用户 ID 错误。

#### Scenario: Sign access token
- **Given** 认证配置包含 JWT secret
- **Given** 调用方提供合法 `user_id`、大于零的 `token_version`、非空 `session_id` 和 TTL
- **When** 调用方使用 `auth.NewJWTService` 创建服务并签发 access token
- **Then** 系统 MUST 返回使用 HMAC SHA-256 签名的 JWT token
- **Then** token claims MUST 包含 `user_id`、`token_version`、`session_id`、subject、expires_at、issuer 和 audience

#### Scenario: Parse valid access token
- **Given** JWT access token 签名有效且未过期
- **Given** token claims 包含合法 `user_id`、大于零的 `token_version` 和非空 `session_id`
- **When** 调用方使用 auth JWT 服务解析该 token
- **Then** 系统 MUST 返回 token claims
- **Then** 系统 MUST 将该 token 识别为 access token

#### Scenario: Reject invalid token subject
- **Given** JWT token 签名有效且未过期
- **Given** token subject 与调用方解析方法要求的 subject 不一致
- **When** 调用方使用 auth JWT 服务解析该 token
- **Then** 系统 MUST 返回无效 subject 错误
- **Then** 系统 MUST NOT 将该 token 视为目标凭证类型

#### Scenario: Reject missing identity fields
- **Given** JWT token 签名有效且未过期
- **Given** token claims 缺少 `user_id`、大于零的 `token_version` 或非空 `session_id`
- **When** 调用方使用 auth JWT 服务解析需要这些字段的 token
- **Then** 系统 MUST 返回对应凭证校验错误
- **Then** 系统 MUST NOT 返回有效 claims

#### Scenario: Reject invalid user id format
- **Given** JWT token 签名有效且未过期
- **Given** token claims 包含非空但不是合法 UUID 的 `user_id`
- **When** 调用方使用 auth JWT 服务解析该 token
- **Then** 系统 MUST 返回无效用户 ID 错误
- **Then** 系统 MUST NOT 返回缺失用户 ID 错误
- **Then** 系统 MUST NOT 返回有效 claims
