## Why

`user-services/internal/service/auth_service.go` 中登录认证、登录后签发策略和改密 token 验证逻辑耦合在较长的 service 方法内，影响后续维护和错误分类审查。当前行为已经由 `user-session-control` 约束，变更目标是在不改变外部 API 契约的前提下让认证成功、状态分支和受限改密凭据验证职责更清晰。

## What Changes

- 将登录流程拆分为专门的用户认证 helper，集中处理 username/password 归一化校验、按 username 查询用户、密码 hash 校验和禁用状态拒绝。
- 保留 `Login()` 对认证成功后的签发策略分支负责，使 `status=300` 的用户继续签发受限改密凭据，`status=100` 的用户继续签发普通 token pair。
- 将改密 token 解析、token version 校验和 `user_id` UUID 解析提取为更精确的 helper，返回 `uuid.UUID` 供 `ChangePassword()` 后续查询用户、校验状态和更新凭证。
- 不改变登录、刷新、改密、退出接口的 HTTP 路径、响应字段、错误码、token TTL、Redis 会话写入或 PostgreSQL 数据模型。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 明确登录认证、认证成功后的签发策略、受限改密凭据验证和改密状态校验的服务内职责边界，保持现有会话控制外部行为不变。

## Impact

- 主要影响 `user-services/internal/service/auth_service.go` 的内部结构和单元测试可读性。
- 相关 capability 为 `user-session-control`，依赖 `user-authentication` 的 JWT claims、认证上下文和 `common/auth` token subject 语义。
- 外部可观察行为保持兼容：无 API 路径、请求/响应结构、错误码、配置项、Redis key、数据库 schema 或 migration 变化。
