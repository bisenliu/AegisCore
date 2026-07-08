## ADDED Requirements

### Requirement: 认证错误应用错误渲染

系统 MUST 将认证、会话、token 和撤销相关稳定错误表达为可由共享 response helper 直接渲染的应用错误，并保持 auth HTTP boundary 无专用 sentinel-to-HTTP 兼容映射。认证错误 MUST 携带稳定 `Kind`、`Reason`、`Code` 和中文公开 `Message`，且 MUST 保持 `errors.Is` 或应用错误 `Reason` 可供登录、refresh 和 logout metrics 分类。

#### Scenario: 无效凭据渲染为未认证响应

- **WHEN** 登录凭据校验返回 `authdomain.ErrInvalidCredentials`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeUnauthenticated`
- **AND** 响应 message MUST 使用当前无效凭据中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `invalid_credentials`
- **AND** 系统 MUST NOT 泄露用户名不存在、密码不匹配或用户状态拒绝的具体细节

#### Scenario: 用户状态拒绝保持无效凭据公开语义

- **WHEN** 登录凭据有效但用户状态不允许普通登录，且错误链包含 `authdomain.ErrUserStatusRejected`
- **THEN** 认证 HTTP 边界 MUST 返回 `401 Unauthorized` 和 `CodeUnauthenticated`
- **AND** 响应 message MUST 继续使用当前无效凭据中文公开文案
- **AND** metrics MUST 能通过 `errors.Is(err, authdomain.ErrUserStatusRejected)` 或稳定 `Reason` 值 `user_status_rejected` 分类该失败
- **AND** 系统 MUST NOT 向客户端暴露具体用户状态

#### Scenario: 缺失认证会话渲染为未认证响应

- **WHEN** 受保护认证 use case 返回 `authdomain.ErrMissingSession`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeUnauthenticated`
- **AND** 响应 message MUST 使用当前登录状态失效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `missing_session`

#### Scenario: token 无效渲染为 token invalid 响应

- **WHEN** token 解析、password change token 校验或受保护认证流程返回 `authdomain.ErrTokenInvalid`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeTokenInvalid`
- **AND** 响应 message MUST 使用当前登录状态失效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `auth_token_invalid`

#### Scenario: refresh session 不存在渲染为 token invalid 响应

- **WHEN** refresh token 对应会话不存在、已退出或已过期，且流程返回 `authdomain.ErrAuthSessionNotFound`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeTokenInvalid`
- **AND** 响应 message MUST 使用当前登录状态失效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `auth_session_not_found`

#### Scenario: refresh session mismatch 渲染为 token invalid 响应

- **WHEN** refresh session 中的 `user_id`、`session_id` 或 `token_version` 与 token claims 不一致，且流程返回 `authdomain.ErrAuthSessionMismatch`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeTokenInvalid`
- **AND** 响应 message MUST 使用当前登录状态失效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `auth_session_mismatch`
- **AND** refresh metrics MUST 继续能分类为 refresh session mismatch

#### Scenario: 强制改密一次性会话无效渲染为 token invalid 响应

- **WHEN** 强制改密流程返回 `authdomain.ErrPasswordChangeSessionNotFound` 或 `authdomain.ErrPasswordChangeSessionMismatch`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeTokenInvalid`
- **AND** 响应 message MUST 使用当前登录状态失效中文公开文案
- **AND** 两类错误 MUST 分别使用稳定 `Reason` 值 `password_change_session_not_found` 和 `password_change_session_mismatch`

#### Scenario: 撤销不完整渲染为服务不可用响应

- **WHEN** 退出当前会话、退出全部会话或安全敏感撤销流程返回 `authdomain.ErrSessionRevocationIncomplete`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `503 Service Unavailable` 和 `CodeServiceUnavailable`
- **AND** 响应 message MUST 使用当前退出登录尚未完全生效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `session_revocation_incomplete`
- **AND** logout metrics MUST 继续能分类撤销不完整失败

#### Scenario: 密码 KDF 繁忙直接渲染为服务不可用响应

- **WHEN** 登录凭据校验返回 `password.ErrPasswordKDFBusy`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `503 Service Unavailable` 和 `CodeServiceUnavailable`
- **AND** 响应 message MUST 使用当前认证服务繁忙中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `password_kdf_busy`
- **AND** 登录 metrics MUST 继续能通过 `errors.Is(err, password.ErrPasswordKDFBusy)` 或稳定 `Reason` 分类为 password KDF busy

#### Scenario: 认证业务错误保留 errors.Is 语义

- **WHEN** auth feature 或测试需要判断认证、会话、token、撤销或 KDF busy 错误
- **THEN** `errors.Is` 对直接返回的应用错误和被包装后的应用错误 MUST 继续支持正确匹配
- **AND** 该匹配语义 MUST NOT 依赖 HTTP transport 层的错误转换函数

### Requirement: 认证 HTTP transport 统一错误出口

auth HTTP transport MUST 对业务 use case 返回错误使用共享 `response.Fail` 入口，避免在 transport 层重复维护 auth domain、identity 或 password 错误到 HTTP 响应的映射。强制改密登录成功分支 MAY 继续使用现有专用 envelope 映射以携带受限 token data，但该路径 MUST NOT 作为错误 mapper 兼容入口。

#### Scenario: 登录业务错误

- **WHEN** `Login` controller 调用登录 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用认证专用错误 mapper
- **AND** controller MUST 保持强制改密成功分支的现有 HTTP `200 OK`、`success=false` 和受限 token data 响应结构

#### Scenario: refresh 业务错误

- **WHEN** `Refresh` controller 调用 refresh use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用认证专用错误 mapper

#### Scenario: 改密业务错误

- **WHEN** `ChangePassword` controller 调用改密 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用认证专用错误 mapper

#### Scenario: 退出当前会话业务错误

- **WHEN** `LogoutCurrentSession` controller 调用退出当前会话 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用认证专用错误 mapper

#### Scenario: 退出全部会话业务错误

- **WHEN** `LogoutAllSessions` controller 调用退出全部会话 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用认证专用错误 mapper

#### Scenario: 不保留认证错误兼容 mapper

- **WHEN** auth HTTP transport 完成本次迁移
- **THEN** 系统 MUST NOT 保留 `toAuthHTTPError`
- **AND** 系统 MUST NOT 新增等价的 sentinel-to-HTTP 兼容函数、跨模块认证错误映射注册表或仅包装 `contracterrors.FromError` 的认证错误函数
