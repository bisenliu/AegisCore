## ADDED Requirements

### Requirement: Authentication sessions use repository abstraction with Redis implementation boundary
用户会话控制能力 SHALL 通过根 `repository.AuthSessionRepository` 抽象管理 token version、Refresh Token 会话和用户活跃会话索引，具体 Redis 实现 MUST 位于 `user-services/internal/repository/redis` 包。service 层 MUST NOT 定义或持有 Redis session store 具体实现。

#### Scenario: Auth service depends on auth session repository abstraction
- **Given** 登录、刷新、退出当前设备、退出全部设备或修改密码流程需要访问会话状态
- **WHEN** auth service 调用会话数据访问层
- **THEN** auth service MUST 依赖 `repository.AuthSessionRepository`
- **THEN** auth service MUST 使用 `repository.AuthSession` 表达会话数据
- **THEN** auth service MUST NOT 依赖 Redis client 或 `repository/redis` 私有实现类型

#### Scenario: Session not found error remains mappable
- **Given** Redis 中不存在指定 Refresh Token 会话记录
- **WHEN** auth service 读取会话
- **THEN** Redis 实现 MUST 返回 `repository.ErrAuthSessionNotFound`
- **THEN** auth service MUST 继续将该错误映射为未认证或 token 无效响应

#### Scenario: Token version mismatch remains mappable
- **Given** token claims 或会话记录中的 `token_version` 与服务端当前版本不一致
- **WHEN** 系统校验 token version
- **THEN** auth session repository MUST 返回 `repository.ErrTokenVersionMismatch`
- **THEN** 系统 MUST 继续拒绝刷新、受保护请求或改密凭据校验

#### Scenario: Redis session storage behavior remains compatible
- **Given** `repository/redis` 承载认证会话 Redis 实现
- **WHEN** 系统创建、读取、删除或批量删除认证会话
- **THEN** Redis key 格式、Refresh Token 会话 TTL、用户活跃会话 ZSet 和过期 member 清理行为 MUST 与迁移前保持一致
- **THEN** token version 缓存未命中时 MUST 继续回源 `repository.UserRepository`

### Requirement: Authentication token validator uses auth session repository abstraction
用户服务运行时 SHALL 将 `repository.AuthSessionRepository` 作为认证中间件 token version validator 的依赖来源。该抽象 MUST 提供 `ValidateTokenVersion(ctx, userID, tokenVersion)`，并保持现有 token version 校验语义。

#### Scenario: Protected route token validation remains compatible
- **Given** 用户服务注册受保护路由认证中间件
- **WHEN** 中间件需要校验 Access Token 的 token version
- **THEN** 中间件 MUST 使用 Fx 注入的 `repository.AuthSessionRepository`
- **THEN** 有效 token MUST 继续允许进入受保护 handler
- **THEN** 版本不一致 token MUST 继续在进入 handler 前被拒绝
