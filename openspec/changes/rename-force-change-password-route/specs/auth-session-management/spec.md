## MODIFIED Requirements

### Requirement: 登录、用途隔离令牌与 HTTP 契约

系统 MUST 在凭证、用户状态和会话策略校验通过后签发用途隔离的 access、refresh 或 password-change token。所有 token MUST 包含标准 `jti`，并通过 `access`、`refresh` 和 `password_change` subject 限定使用流程；任一用途、subject 或必要 claims 不匹配时 MUST 被拒绝。系统 MUST 仅通过 `/api/v1/auth` 暴露认证入口，并使用共享 response helper 渲染业务错误。强制改密 HTTP 入口 MUST 使用 `POST /api/v1/auth/force-change-password`，MUST NOT 暴露旧 `POST /api/v1/auth/change-password` 路径。

#### Scenario: 普通登录成功

- **WHEN** 用户提供合法用户名和正确密码，且状态允许普通登录
- **THEN** 系统 MUST 创建普通 refresh session 并签发 access token 与 refresh token
- **AND** 登录结果 MUST 为 `PasswordChangeRequired=false`
- **AND** HTTP 响应 MUST 为 `200 OK`、`CodeOK` 和 `success=true`，data MUST 包含 access token、refresh token、token type 和 access token 过期秒数，MUST NOT 包含登录状态枚举字段

#### Scenario: 登录拒绝与侧信道防护

- **WHEN** 用户名不存在、密码不匹配，或用户状态不允许登录且不属于强制改密状态
- **THEN** 系统 MUST 拒绝签发任何 token 和创建会话，且公开错误 MUST NOT 泄露用户是否存在、密码匹配结果或具体用户状态
- **AND** 用户名不存在时 MUST 使用当前 bcrypt dummy hash 执行 dummy password verification
- **AND** 旧 Argon2id、未知算法或格式非法的存储哈希 MUST 被视为无效凭据，MUST NOT 触发旧哈希验证、迁移、fallback 或 rehash

#### Scenario: 强制改密登录响应

- **WHEN** 凭据有效且用户状态要求强制修改密码
- **THEN** 登录结果 MUST 为 `PasswordChangeRequired=true`，系统 MUST 创建一次性 password-change session 并只签发 subject 为 `password_change` 的受限 token，MUST NOT 创建普通 refresh session 或签发 refresh token
- **AND** HTTP 响应 MUST 为 `200 OK`、`CodePasswordChangeRequired`、code `20006` 和 `success=false`
- **AND** data MUST 只包含受限 access token、token type 和过期秒数，MUST NOT 包含 refresh token、`status`、`authenticated` 或 `password_change_required` 枚举字段

#### Scenario: 路由保护与最小 verifier

- **WHEN** 调用方访问登录、refresh 或强制改密入口
- **THEN** controller MUST 允许请求进入并在业务层校验相应凭据或 token
- **AND** 强制改密入口 MUST 挂载为 `POST /api/v1/auth/force-change-password`，MUST 使用 `password_change` 受限 token 和一次性 password-change session 完成校验，MUST NOT 要求普通 access token middleware 先放行
- **AND** 退出当前会话、退出全部会话和普通改密 MUST 在业务处理前校验 bearer token、user-service access claims 和 token version
- **AND** 系统 MUST NOT 暴露旧认证路径别名或认证绕过路径，包括 `POST /api/v1/auth/change-password`
- **AND** 共享认证 middleware 只 MUST 获得 access token 验证能力，MUST NOT 获得 refresh token、password-change token 或任何 token 签发能力

#### Scenario: 认证错误统一出口

- **WHEN** auth controller 收到凭据、用户状态、token 或 session 无效错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)` 返回稳定的 `401 Unauthorized` 认证错误和统一公开文案
- **WHEN** 认证流程返回 `authdomain.ErrSessionRevocationIncomplete`
- **THEN** 系统 MUST 返回 `503 Service Unavailable`、`CodeServiceUnavailable` 和稳定公开消息
- **AND** 认证错误 MUST 携带稳定 `Kind`、`Reason`、`Code` 和中文公开 `Message`，直接或包装后 MUST 保持 `errors.Is` 或稳定 `Reason` 分类能力
- **AND** HTTP transport MUST NOT 维护认证专用 sentinel-to-HTTP mapper，响应 MUST NOT 泄露 Redis key、session ID、jti、SQL、stacktrace 或内部错误文本
