## MODIFIED Requirements

### Requirement: Refresh access tokens through revocable refresh sessions
系统 SHALL 提供 Refresh Token 刷新能力。刷新时，系统 MUST 校验 Refresh Token 签名、过期时间和 Refresh Token subject，解析外部用户标识和会话标识，确认 Redis 会话仍存在，并校验该用户当前 `token_version` 与 Refresh Token 或会话记录中的版本一致；校验通过后签发新的 Access Token。系统 SHOULD 对 Refresh Token 执行轮转，使旧 Refresh Token 对应会话失效并创建新会话。启用 Refresh Token 轮转时，系统 MUST 确保成功响应中返回的新 Refresh Token 对应的 Redis 会话已经可用，并且 MUST NOT 在新 token 签发失败或新 Redis 会话创建失败时提前撤销已通过校验的旧 Refresh Token 会话。Access Token 与 Refresh Token 的 subject/token type 枚举 MUST 由 `common/security/auth` 统一提供，签发方法 MUST 内部强制设置对应 subject。刷新流程 MUST 通过 `common/security/auth.StripBearerPrefix` 统一处理请求体中可选 Bearer 前缀。

#### Scenario: Refresh succeeds for active session
- **Given** Refresh Token 签名有效且未过期
- **Given** Refresh Token claims 包含 UUID `user_id`
- **Given** Redis 中存在对应会话记录
- **Given** 用户当前 `token_version` 与会话版本一致
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 签发新的 Access Token
- **Then** 新 Access Token MUST 包含当前 UUID `user_id`、`token_version` 和有效 `session_id`

#### Scenario: Refresh accepts optional bearer prefix in request body
- **Given** Refresh Token 签名有效且未过期
- **Given** Redis 中存在对应会话记录
- **When** 调用方在刷新请求体 `refresh_token` 字段中提交 `Bearer <refresh-token>`
- **Then** 系统 MUST 通过 `common/security/auth.StripBearerPrefix` 剥离 `Bearer ` 前缀
- **Then** 系统 MUST 按剥离后的 Refresh Token 执行刷新校验

#### Scenario: Refresh request body prefers raw token
- **Given** 调用方查看刷新接口 DTO 或 Swagger 文档
- **When** 文档展示 `refresh_token` 字段示例
- **Then** 示例值 SHOULD 使用裸 Refresh Token
- **Then** 文档 MUST NOT 要求请求体字段必须带 `Bearer ` 前缀

#### Scenario: Refresh rejects revoked session
- **Given** Refresh Token 签名有效且未过期
- **Given** Redis 中不存在对应会话记录
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 返回未认证或 token 无效响应
- **Then** 系统 MUST NOT 签发新的 Access Token

#### Scenario: Refresh rejects access token subject
- **Given** Access Token 签名有效且未过期
- **When** 调用方将该 Access Token 作为刷新请求体 `refresh_token` 字段提交
- **Then** 系统 MUST 因 subject 不是 Refresh Token subject 而拒绝刷新
- **Then** 系统 MUST NOT 签发新的 Access Token

#### Scenario: Refresh token signer sets refresh subject
- **Given** 调用方使用公共 JWT service 签发 Refresh Token
- **When** `SignRefreshToken` 返回 token
- **Then** token claims subject MUST 等于公共 Refresh Token subject 枚举
- **Then** 调用方 MUST NOT 通过字符串入参覆盖该 subject

#### Scenario: Refresh rejects changed token version
- **Given** Refresh Token 签名有效且未过期
- **Given** Redis 中存在对应会话记录
- **Given** PostgreSQL 或版本缓存中的当前 `token_version` 与会话版本不一致
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 拒绝刷新
- **Then** 系统 MUST NOT 签发新的 Access Token

#### Scenario: Refresh token rotation revokes old session after new session is usable
- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 创建新的会话标识或刷新会话记录
- **Then** 系统 MUST 确保新的 Refresh Token 对应的 Redis 会话已经可用
- **Then** 系统 MUST 撤销旧 Refresh Token 会话记录
- **Then** 系统 MUST 返回新的 Refresh Token

#### Scenario: Rotation keeps old session when new token signing fails
- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **Given** 新 token 签发失败
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 撤销已通过校验的旧 Refresh Token 会话记录
- **Then** 调用方后续 MAY 使用旧 Refresh Token 重试刷新

#### Scenario: Rotation keeps old session when new session creation fails
- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **Given** 新 Redis 会话创建失败
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 撤销已通过校验的旧 Refresh Token 会话记录
- **Then** 系统 MUST NOT 返回没有可用 Redis 会话支撑的新 Refresh Token

#### Scenario: Rotation does not expose new token when old session revocation fails
- **Given** Refresh Token 轮转已启用
- **Given** 新 token 已签发
- **Given** 新 Redis 会话已创建
- **Given** 旧 Refresh Token 会话撤销失败
- **When** 系统处理刷新响应
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 向调用方返回新 Refresh Token
- **Then** 系统 SHOULD 尝试清理已创建但未返回给调用方的新 Redis 会话

#### Scenario: Strict replay prevention uses atomic rotation
- **Given** Refresh Token 轮转已启用
- **Given** 系统安全目标要求防止同一个旧 Refresh Token 被并发重放
- **When** 系统执行 Refresh Token 轮转
- **Then** 系统 MUST 使用 Redis 事务、Lua 脚本或等价机制原子完成旧会话校验、新会话创建和旧会话撤销
- **Then** 系统 MUST 避免命令间失败或并发刷新造成旧会话和新会话同时可用或同时不可用的中间状态
