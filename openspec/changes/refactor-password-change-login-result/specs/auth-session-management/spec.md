## MODIFIED Requirements

### Requirement: 用户登录与令牌签发

系统 MUST 提供用户名密码登录能力，并在凭证、用户状态和会话策略校验通过后签发访问令牌与刷新令牌。登录 use case MUST 使用登录专属结果字段表达是否需要强制改密；登录失败仍 MUST 通过错误返回。系统 MUST 将密码 KDF 资源池繁忙视为临时服务不可用，而不是无效凭据。

#### Scenario: 登录成功

- **WHEN** 用户提供合法用户名和正确密码，且用户状态允许普通登录
- **THEN** 系统 MUST 创建普通 refresh session、签发 access token 与 refresh token
- **AND** 登录 use case MUST 返回 `PasswordChangeRequired=false`
- **AND** 登录响应 MUST 返回 HTTP `200 OK`
- **AND** 登录响应 envelope MUST 携带 `CodeOK`
- **AND** 登录响应 envelope 的 `success` MUST 为 `true`
- **AND** 登录响应 data MUST 携带 access token、refresh token、token type 和 access token 过期秒数
- **AND** 登录响应 data MUST NOT 携带登录状态枚举字段

#### Scenario: 凭证错误

- **WHEN** 用户名不存在或密码不匹配
- **THEN** 系统 MUST 拒绝登录并返回一致的认证错误，且 MUST NOT 泄露具体凭证匹配细节

#### Scenario: 用户状态禁止登录

- **WHEN** 用户存在但状态不允许登录，且该状态不是强制改密状态
- **THEN** 系统 MUST 拒绝签发令牌并返回明确的状态相关错误

#### Scenario: 强制改密用户登录

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 系统 MUST 只签发 subject 为 `password_change` 的受限 token，不得创建普通 refresh session，也不得返回 refresh token
- **AND** 登录 use case MUST 返回 `PasswordChangeRequired=true`，而不是通过 error 表达该分支

#### Scenario: 强制改密登录返回业务码 envelope

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 登录响应 MUST 返回 HTTP `200 OK`
- **AND** 登录响应 envelope MUST 携带 `CodePasswordChangeRequired`
- **AND** 登录响应 envelope 的 `code` MUST 为 `20006`
- **AND** 登录响应 envelope 的 `success` MUST 为 `false`
- **AND** 登录响应 envelope 的 `message` MUST 使用强制改密用户提示
- **AND** 登录响应 data MUST 携带 subject 为 `password_change` 的受限 token 数据
- **AND** 登录响应 MUST NOT 携带 refresh token
- **AND** 登录响应 data MUST NOT 携带 `status`、`authenticated` 或 `password_change_required` 枚举字段

#### Scenario: 普通登录仍返回成功 code

- **WHEN** 用户凭据有效且账号状态允许普通登录
- **THEN** 登录响应 envelope MUST 携带 `CodeOK`
- **AND** 登录响应 envelope 的 `success` MUST 为 `true`
- **AND** 登录响应 MUST 携带 access token 与 refresh token
- **AND** 登录响应 MUST NOT 携带 `CodePasswordChangeRequired`

#### Scenario: 强制改密分支不创建普通会话

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 系统 MUST 只签发 subject 为 `password_change` 的受限 token
- **AND** 系统 MUST NOT 创建普通 refresh session
- **AND** 系统 MUST NOT 签发 refresh token

#### Scenario: 密码 KDF 资源繁忙

- **WHEN** 登录凭据校验进入密码 KDF 但实例内 Argon2 执行和等待队列已达资源上限
- **THEN** 系统 MUST 拒绝本次登录并返回 `503 Service Unavailable`
- **AND** 系统 MUST NOT 将该错误映射为无效凭据
- **AND** 系统 MUST NOT 签发 access token、refresh token 或 password change token
- **AND** 系统 MUST NOT 泄露用户名存在性、密码匹配状态、队列长度或 Argon2 并发配置

#### Scenario: token 缺少 jti

- **WHEN** access token、refresh token 或 password change token 缺少标准 `jti`
- **THEN** token MUST 被拒绝

#### Scenario: token subject 不匹配

- **WHEN** subject 为 `access`、`refresh` 或 `password_change` 的 token 被用于不匹配的认证流程
- **THEN** 系统 MUST 拒绝该 token，且 MUST NOT 在三类 token 之间兼容复用

### Requirement: 认证会话 E2E flow 断言规范

系统 MUST 使用语义化断言覆盖 user-service E2E HTTP flow 中的认证会话行为，包括普通登录、强制改密登录、修改密码、旧密码登录失败、登出当前会话和 refresh token 失效。断言迁移 MUST 保持当前认证会话、token、错误码和 response envelope 语义不变，且 MUST 以 envelope `CodePasswordChangeRequired` 作为强制改密登录分支的当前语义。

#### Scenario: 普通登录 token 断言

- **WHEN** E2E flow 使用合法用户名和密码完成普通登录
- **THEN** 测试 MUST 使用 `require.NotEmpty`、`require.Equal`、`require.Greater` 或必要 `assert` 验证 access token、refresh token、token type 和 expires_in
- **AND** 测试 MUST NOT 接受缺失 refresh token、旧 token type、旧错误码或旧响应字段兼容分支

#### Scenario: 强制改密登录断言

- **WHEN** E2E flow 使用强制改密用户凭据登录
- **THEN** 测试 MUST 使用语义化断言验证 HTTP `200 OK`、`success=false`、`CodePasswordChangeRequired`、受限 access token metadata 和空 refresh token
- **AND** 测试 MUST NOT 接受 `success=true`、`CodeOK`、响应 data 状态枚举或旧 `password_change_required` 兼容字段

#### Scenario: 改密、登出和 refresh 失败断言

- **WHEN** E2E flow 完成改密、使用旧密码重试登录、登出当前会话并使用旧 refresh token 刷新
- **THEN** 测试 MUST 使用语义化断言验证改密成功、旧密码认证失败、登出成功和 refresh token 失效的当前 HTTP status 与应用错误码
- **AND** 迁移 MUST NOT 改变 refresh session、token version、password change token 或 logout 运行时语义

## ADDED Requirements

### Requirement: 登录结果分支模型

Auth application MUST 使用登录 use case 专属结果表达普通登录和强制改密登录分支。`TokenResult` MUST 只表达 token 载荷本身；登录业务分支 MUST 位于 `LoginResult` 或等价登录 use case 专属结果类型中，避免 token issuer 或 transport 通过 token 载荷推断业务分支。

#### Scenario: 普通登录结果

- **WHEN** 登录 use case 完成普通登录并创建 refresh session
- **THEN** 返回结果 MUST 包含 `PasswordChangeRequired=false` 和普通 token 载荷
- **AND** token 载荷 MUST 包含 access token、refresh token、token type 和 access token 过期秒数

#### Scenario: 强制改密登录结果

- **WHEN** 登录 use case 完成强制改密登录并创建一次性 password change session
- **THEN** 返回结果 MUST 包含 `PasswordChangeRequired=true` 和受限 password change token 载荷
- **AND** token 载荷 MUST NOT 包含 refresh token
- **AND** token 载荷 MUST NOT 通过 `PasswordChangeRequired` 或等价字段表达业务分支
- **AND** 返回结果 MUST NOT 暴露 `authenticated` 或 `password_change_required` 字符串枚举

#### Scenario: token issuer 保持载荷职责

- **WHEN** token issuer 签发普通 token pair 或 password change token
- **THEN** token issuer MUST 返回 transport-neutral token 载荷
- **AND** token issuer MUST NOT 决定登录 HTTP 响应 envelope、登录业务状态 code 或强制改密响应 shape

### Requirement: 强制改密登录 HTTP 响应

Auth HTTP transport MUST 将强制改密登录表达为业务码 envelope，并使用受限 token 载荷作为 data。controller MUST 使用专用 mapper 生成该 envelope，普通登录 MUST 继续使用普通成功响应。

#### Scenario: controller 映射普通登录

- **WHEN** 登录 use case 返回普通登录结果
- **THEN** controller MUST 返回 HTTP `200 OK`、`CodeOK`、`success=true` 和普通登录响应 DTO
- **AND** 响应 DTO MUST 包含 access token、refresh token、token type 和 expires_in
- **AND** 响应 DTO MUST NOT 包含 `status` 字段

#### Scenario: controller 映射强制改密登录

- **WHEN** 登录 use case 返回强制改密结果
- **THEN** controller MUST 返回 HTTP `200 OK`、`CodePasswordChangeRequired`、`success=false` 和强制改密 envelope
- **AND** envelope data MUST 包含 access token、token type 和 expires_in
- **AND** envelope data MUST NOT 包含 refresh token
- **AND** controller MUST 调用 `toPasswordChangeRequiredEnvelope` 或等价专用 mapper

#### Scenario: 不保留响应枚举

- **WHEN** auth HTTP transport 完成本次重构
- **THEN** 系统 MUST NOT 在登录响应 data 中返回 `status` 字段
- **AND** 系统 MUST NOT 暴露 `authenticated` 或 `password_change_required` 响应枚举
- **AND** 系统 MUST NOT 为登录 token 载荷保留与 `TokenResponse` 字段完全重复的 `LoginResponse` DTO

#### Scenario: 登录失败仍走错误出口

- **WHEN** 登录 use case 返回凭证错误、用户状态拒绝、KDF busy 或系统错误
- **THEN** controller MUST 继续通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 系统 MUST NOT 用登录结果字段表达失败分支
