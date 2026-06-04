## ADDED Requirements

### Requirement: Validate and normalize auth request input before service business flow
系统 MUST 在 Auth Service 执行认证会话业务编排前完成认证请求的请求级清洗和基础校验。登录请求的 `username` 和 `password` 裁剪、空凭据校验，改密请求的 `new_password` 裁剪和空值校验，以及刷新请求体 `refresh_token` 的可选 Bearer 前缀剥离和空值校验 MUST 位于 Controller、共享请求校验器或服务内 Validation 层，而不是作为 Auth Service 的主要职责。

#### Scenario: Normalize login credentials before authentication
- **Given** 调用方提交登录请求且 `username` 或 `password` 前后包含空白
- **When** controller 处理登录请求并调用 Auth Service
- **Then** 空白裁剪 MUST 在 Controller 或服务内 Validation 层完成
- **Then** Auth Service MUST 使用已规范化的 `username` 和明文密码执行凭据认证

#### Scenario: Reject empty login credentials before repository lookup
- **Given** 登录请求的 `username` 或 `password` 在裁剪后为空
- **When** controller 处理登录请求
- **Then** 请求 MUST 在查询用户资料、校验密码或签发 token 前被判定为认证失败或请求校验失败
- **Then** 系统 MUST NOT 创建 Redis 会话记录
- **Then** Auth Service MUST NOT 将空凭据基础校验作为登录流程的主要业务分支

#### Scenario: Normalize password change input before credential update
- **Given** 调用方提交改密请求且 `new_password` 前后包含空白
- **When** controller 处理改密请求并调用 Auth Service
- **Then** 新密码空白裁剪和裁剪后空值校验 MUST 在 Controller 或服务内 Validation 层完成
- **Then** Auth Service MUST 使用已规范化的新密码执行密码哈希和凭证更新流程

#### Scenario: Normalize refresh token request before refresh flow
- **Given** 调用方在刷新请求体 `refresh_token` 字段提交裸 Refresh Token 或 `Bearer <refresh-token>`
- **When** controller 处理刷新请求并调用 Auth Service
- **Then** 请求体 token 的可选 Bearer 前缀剥离和空值校验 MUST 在 Controller 或服务内 Validation 层完成
- **Then** Auth Service MUST 使用已规范化的 Refresh Token 执行 token claims、session 和 token version 校验

### Requirement: Keep authentication and session semantics in Auth Service
系统 MUST 保持认证会话安全语义由 Auth Service 或认证中间件边界负责。Validation 层 MUST NOT 执行依赖 JWT claims、Redis session、token version、用户状态或 Repository 查询的认证业务校验。

#### Scenario: Auth service verifies credentials after request validation
- **Given** 登录请求已经完成请求级规范化和基础校验
- **When** Auth Service 执行登录流程
- **Then** Auth Service MUST 查询用户认证资料
- **Then** Auth Service MUST 使用共享密码 helper 校验 `password_hash`
- **Then** Auth Service MUST 根据用户状态决定拒绝登录、签发普通 token 或签发受限改密凭据

#### Scenario: Auth service verifies password-change token semantics
- **Given** 改密请求已经完成请求级规范化和基础校验
- **When** Auth Service 执行改密流程
- **Then** Auth Service MUST 解析 password-change token claims
- **Then** Auth Service MUST 校验服务端当前 `token_version` 与 claims 一致
- **Then** Auth Service MUST 查询用户状态并只允许 `status=300` 用户完成改密

#### Scenario: Auth service verifies refresh session semantics
- **Given** 刷新请求已经完成请求级规范化和基础校验
- **When** Auth Service 执行刷新流程
- **Then** Auth Service MUST 校验 Refresh Token 签名、subject、claims user_id 和 session_id
- **Then** Auth Service MUST 校验 Redis session 存在且与 claims 匹配
- **Then** Auth Service MUST 校验当前 `token_version` 与会话版本一致后才签发新 token

#### Scenario: Auth context validation remains security boundary
- **Given** 调用方请求退出当前设备或退出全部设备
- **When** Auth Service 从认证上下文读取 `user_id` 和 `session_id`
- **Then** Auth Service 或认证中间件 MUST 继续校验上下文中的认证身份和会话标识
- **Then** 普通请求 Validation 层 MUST NOT 替代认证上下文安全校验
