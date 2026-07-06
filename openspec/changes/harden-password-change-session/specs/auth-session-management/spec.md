## ADDED Requirements

### Requirement: 强制改密一次性会话

系统 MUST 为强制改密流程创建服务端一次性 password-change session，并将 `password_change` token 的 `jti`、`session_id`、`user_id` 和 `token_version` 与 Redis 状态绑定。password-change session MUST 使用独立短 TTL，并 MUST NOT 复用 refresh session 存储语义、refresh session 上限裁剪或 refresh token TTL。

#### Scenario: 强制改密登录创建一次性会话
- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 系统 MUST 签发 subject 为 `password_change` 的受限 token
- **AND** 系统 MUST 创建与该 token `jti`、`session_id`、`user_id` 和 `token_version` 一致的 Redis password-change session
- **AND** 系统 MUST NOT 创建普通 refresh session
- **AND** 系统 MUST NOT 返回 refresh token

#### Scenario: 一次性会话创建失败
- **WHEN** 系统无法创建 Redis password-change session
- **THEN** 登录 MUST 失败
- **AND** 系统 MUST NOT 向客户端返回已签发的 password-change token

#### Scenario: 独立改密 token TTL
- **WHEN** 系统签发 password-change token 或创建 password-change session
- **THEN** token 和 session MUST 使用 `auth.jwt.password_change_token_ttl`
- **AND** 系统 MUST NOT 使用 `auth.jwt.access_token_ttl` 或 `auth.jwt.refresh_token_ttl` 作为 password-change token TTL

#### Scenario: 非正数改密 token TTL
- **WHEN** `auth.jwt.password_change_token_ttl` 未配置、配置为 `0` 或配置为负数
- **THEN** 系统 MUST 使用 5 分钟作为默认 password-change token TTL
- **AND** 系统 MUST NOT 创建无过期时间的 password-change token 或 password-change session

### Requirement: 强制改密 token 原子消费

系统 MUST 在更新密码前原子消费 password-change session。消费 MUST 同时校验 `jti`、`session_id`、`user_id` 和 `token_version`，任一不匹配、会话不存在、会话过期、会话已撤销或会话已被消费时，系统 MUST 返回统一无效凭据错误，且 MUST NOT 泄露具体失败原因。

#### Scenario: 首次消费成功
- **WHEN** 调用方提交有效且未过期的 password-change token，且 Redis password-change session 与 token claims 完全一致
- **THEN** 系统 MUST 原子删除该 password-change session
- **AND** 系统 MAY 继续执行密码更新

#### Scenario: 重复消费被拒绝
- **WHEN** 同一个 password-change token 已被成功消费后再次用于改密
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 再次更新密码或递增 `token_version`

#### Scenario: 过期会话被拒绝
- **WHEN** password-change token 未通过 JWT 过期校验，或对应 Redis password-change session 已过期
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 更新密码或递增 `token_version`

#### Scenario: 撤销会话被拒绝
- **WHEN** password-change session 已被服务端撤销或删除
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 更新密码或递增 `token_version`

#### Scenario: claims 不一致被拒绝
- **WHEN** password-change token 中的 `jti`、`session_id`、`user_id` 或 `token_version` 与 Redis password-change session 不一致
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 更新密码或递增 `token_version`

#### Scenario: 并发消费只有一个成功
- **WHEN** 多个请求并发使用同一个有效 password-change token 执行改密
- **THEN** 系统 MUST 最多允许一个请求成功消费 password-change session
- **AND** 其他请求 MUST 返回统一无效凭据
- **AND** 系统 MUST 最多执行一次密码更新和一次 `token_version` 递增

### Requirement: 强制改密凭据条件更新

系统 MUST 在强制改密更新凭据时校验用户仍处于强制改密状态，且当前 PostgreSQL `token_version` 与 password-change token claims 中的旧版本一致。条件不满足时，系统 MUST 返回统一无效凭据，并 MUST NOT 更新密码、状态或 `token_version`。

#### Scenario: 状态和版本匹配时更新
- **WHEN** password-change session 已成功消费，用户仍处于强制改密状态，且 PostgreSQL `token_version` 等于 token claims 中的旧版本
- **THEN** 系统 MUST 更新密码哈希
- **AND** 系统 MUST 将用户状态恢复为正常
- **AND** 系统 MUST 递增 `token_version`

#### Scenario: 状态不再要求改密
- **WHEN** password-change session 已成功消费，但用户状态不再要求强制改密
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 更新密码或递增 `token_version`

#### Scenario: 旧 token version 不匹配
- **WHEN** password-change session 已成功消费，但 PostgreSQL 当前 `token_version` 不等于 token claims 中的旧版本
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 更新密码或递增 `token_version`

### Requirement: 强制改密后安全撤销结果

系统 MUST 在强制改密成功更新凭据后刷新 token version 投影、失效本地 token version cache 并删除该用户 refresh sessions。任一撤销投影步骤失败时，系统 MUST 返回可观察的安全撤销未完成错误，MUST NOT 返回普通 `Changed: true` 成功结果。

#### Scenario: 撤销全部成功
- **WHEN** 强制改密凭据更新成功，且本地 token version cache 失效、Redis token version 投影刷新和 refresh session 删除均成功
- **THEN** 系统 MUST 返回改密成功结果
- **AND** 旧 access token MUST 因 `token_version` 不匹配而无法继续访问受保护资源
- **AND** 旧 refresh token MUST 无法继续刷新

#### Scenario: token version 投影失败
- **WHEN** 强制改密凭据更新成功，但 Redis token version 投影刷新失败
- **THEN** 系统 MUST 尝试删除 Redis token version 投影
- **AND** 系统 MUST 返回安全撤销未完成错误
- **AND** 系统 MUST NOT 返回普通 `Changed: true` 成功结果

#### Scenario: refresh session 删除失败
- **WHEN** 强制改密凭据更新成功，但删除该用户 refresh sessions 失败
- **THEN** 系统 MUST 返回安全撤销未完成错误
- **AND** 系统 MUST NOT 返回普通 `Changed: true` 成功结果

#### Scenario: 本地 token version cache 失效失败
- **WHEN** 强制改密凭据更新成功，但本实例本地 token version cache 失效失败
- **THEN** 系统 MUST 返回安全撤销未完成错误
- **AND** 系统 MUST NOT 返回普通 `Changed: true` 成功结果

#### Scenario: HTTP 错误映射
- **WHEN** 强制改密返回安全撤销未完成错误
- **THEN** 认证 HTTP 边界 MUST 返回 `503 Service Unavailable`
- **AND** 响应 MUST 表达认证服务暂时无法完成安全撤销
- **AND** 响应 MUST NOT 泄露 Redis key、session ID、jti、SQL、stacktrace 或内部错误文本
