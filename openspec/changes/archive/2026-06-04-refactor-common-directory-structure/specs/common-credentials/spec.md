## ADDED Requirements

### Requirement: Host credential primitives under common security path
共享凭证原语 SHALL 位于 `common/security` 分类目录下。JWT token 凭证服务、Bearer 认证传输常量和认证上下文 helper MUST 位于 `common/security/auth`；Argon2id 密码 hash 与校验 MUST 位于 `common/security/password`。目录迁移 MUST 保持 JWT claims、token subject、Bearer 解析、认证上下文、密码 hash 格式和密码校验语义不变。

#### Scenario: Auth primitives move without token behavior changes
- **WHEN** JWT 服务和认证上下文 helper 迁移到 `common/security/auth`
- **THEN** access token、refresh token 和密码变更 token 的签发与解析语义 MUST 保持不变
- **THEN** Authorization header、Bearer token 类型、Bearer 前缀和认证上下文读取写入行为 MUST 保持不变

#### Scenario: Password primitives move without hash behavior changes
- **WHEN** 密码 helper 迁移到 `common/security/password`
- **THEN** Argon2id hash 输出格式、默认参数、空密码错误和无效 hash 错误语义 MUST 保持不变
- **THEN** 密码校验 MUST 继续使用 encoded hash 中的参数重新计算并执行 constant-time comparison

#### Scenario: Security primitives remain side-effect free
- **WHEN** 服务或测试导入 `common/security/auth` 或 `common/security/password`
- **THEN** 这些包 MUST NOT 创建 Redis client、PostgreSQL 连接池、Ent client、HTTP server 或 Fx lifecycle
- **THEN** 用户服务登录、刷新、登出和 session repository 逻辑 MUST 继续由 `user-services` 自己拥有
