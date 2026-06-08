## ADDED Requirements

### Requirement: Authentication API contracts are grouped by capability

用户会话控制能力 SHALL 使用按业务能力组织的认证 API 契约包承载登录、刷新、强制改密、登出和 token 响应模型。实现 MUST NOT 继续依赖全局 `user-services/internal/dto` 包表达认证会话契约，并 MUST 保持认证相关 HTTP API、token 语义、Redis 会话行为和响应结构不变。

#### Scenario: Auth contract types use auth API package
- **WHEN** controller、service、validation 或测试引用登录请求、刷新请求、改密请求、token 响应、改密响应或登出响应
- **THEN** 这些引用 MUST 来自认证 API 契约包
- **THEN** 这些引用 MUST NOT 来自全局 `internal/dto` 包

#### Scenario: Login and refresh contracts remain compatible
- **WHEN** 认证请求和响应类型迁移完成
- **THEN** 登录请求 MUST 继续使用 `username` 和 `password` 字段
- **THEN** 刷新请求 MUST 继续使用 `refresh_token` 字段并兼容裸 token 或 Bearer 值
- **THEN** token 响应 MUST 继续包含 `access_token`、可选 `refresh_token`、`token_type`、`expires_in` 和可选 `password_change_required`

#### Scenario: Password change and logout contracts remain compatible
- **WHEN** 改密和登出响应类型迁移完成
- **THEN** 改密请求 MUST 继续从 Authorization header 接收受限 token 并从 JSON 请求体接收 `new_password`
- **THEN** 改密响应 MUST 继续使用 `changed` 字段表达完成状态
- **THEN** 登出响应 MUST 继续使用 `logged_out` 字段表达完成状态

#### Scenario: Auth service semantics remain unchanged
- **WHEN** 认证 API 契约类型迁移完成
- **THEN** 登录、刷新、强制改密、退出当前设备和退出全部设备的认证边界、token claims、token version 校验、Redis 会话生命周期和失败响应语义 MUST 保持不变
