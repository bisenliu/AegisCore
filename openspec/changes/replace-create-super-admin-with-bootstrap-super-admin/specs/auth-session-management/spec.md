## MODIFIED Requirements

### Requirement: 强制改密一次性流程

系统 MUST 为强制改密 token 创建服务端一次性 password-change session，并在更新密码前原子消费该 session。token 与 session MUST 使用独立短 TTL，并绑定 `jti`、`session_id`、`user_id` 和 `token_version`；MUST NOT 复用 refresh session 的 TTL、存储语义或上限裁剪策略。RBAC bootstrap 创建的初始超级管理员用户 MUST 通过同一强制改密流程完成首次密码变更，bootstrap CLI MUST NOT 直接实现认证撤销逻辑。

#### Scenario: 创建一次性会话

- **WHEN** 强制改密登录签发 password-change token
- **THEN** 系统 MUST 创建与 token claims 完全一致的 Redis password-change session
- **AND** token 和 session MUST 使用 `auth.jwt.password_change_token_ttl`
- **AND** 该配置未设置或非正数时 MUST 使用 5 分钟默认 TTL，MUST NOT 创建无过期时间的 token 或 session
- **AND** session 创建失败时登录 MUST 失败，已签发 token MUST NOT 返回客户端

#### Scenario: bootstrap 用户首次登录

- **WHEN** RBAC bootstrap 创建的固定超级管理员用户使用临时密码首次登录
- **THEN** 用户状态 MUST 为 `identity.UserStatusMustChangePassword` 并只获得 subject 为 `password_change` 的受限 token
- **AND** 系统 MUST NOT 创建普通 refresh session 或签发 refresh token
- **AND** 完成强制改密后用户状态 MUST 变为 normal，随后才能正常登录并使用超级管理员权限
- **AND** bootstrap CLI MUST NOT 直接执行条件凭据更新、token version 更新、Redis 投影刷新、本地缓存失效或 refresh session 撤销

#### Scenario: 原子消费和并发保护

- **WHEN** token 有效、未过期且 Redis session 的 `jti`、`session_id`、`user_id` 和 `token_version` 全部匹配
- **THEN** 系统 MUST 原子删除一次性 session，并 MAY 继续执行凭据条件更新
- **WHEN** 多个请求并发使用同一个有效 password-change token
- **THEN** 系统 MUST 最多允许一个请求成功消费 session、更新密码并递增一次 `token_version`
- **AND** 其他请求 MUST 返回统一无效凭据

#### Scenario: 一次性凭据无效

- **WHEN** token 或 session 过期、不存在、已撤销、已消费，或任一绑定 claims 不一致
- **THEN** 系统 MUST 返回统一无效凭据错误
- **AND** 系统 MUST NOT 泄露具体失败原因，也 MUST NOT 更新密码、状态或 `token_version`

#### Scenario: 条件更新和撤销

- **WHEN** session 已消费，用户仍为 `UserStatusMustChangePassword` 且 PostgreSQL 当前 `token_version` 等于 token 中旧版本
- **THEN** 系统 MUST 更新密码哈希、将状态改为 `UserStatusNormal` 并递增 `token_version`
- **AND** 状态或版本不匹配时系统 MUST 返回统一无效凭据，并 MUST NOT 更新密码、状态或版本
- **AND** 凭据更新成功后系统 MUST 失效本地 token version cache、刷新 Redis 投影并删除该用户 refresh sessions
- **AND** 任一步失败 MUST 返回可观察的安全撤销未完成错误，MUST NOT 返回普通改密成功结果
