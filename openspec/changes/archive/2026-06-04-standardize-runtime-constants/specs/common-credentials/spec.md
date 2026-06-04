## ADDED Requirements

### Requirement: Keep credential constants with credential primitives
凭证相关常量 SHALL 位于拥有对应凭证原语的包内。Authorization header、Bearer token 类型、Bearer 前缀、认证上下文 key、JWT subject、JWT claim 字段和密码 hash 参数 MUST 由 `common/auth` 或 `common/password` 维护；用户服务认证业务 TTL fallback 和 session repository key 格式 MUST 保持在用户服务认证能力边界内。

#### Scenario: Authentication transport constants are reused
- **WHEN** middleware、controller、DTO 或 Swagger-adjacent 代码需要表达 Authorization header 或 Bearer token 类型
- **THEN** Go 代码中的运行时逻辑 MUST 复用 `common/auth` 的认证传输常量
- **THEN** Swagger annotation 或 example 中无法引用 Go 常量的字面量 MUST 与公共认证传输常量保持一致

#### Scenario: JWT claim and subject constants stay in common auth
- **WHEN** 实现整理 JWT access、refresh 或 password-change token 的 subject 与 claim 字段
- **THEN** subject 和 claim 常量 MUST 由 `common/auth` 维护
- **THEN** 实现 MUST NOT 将 JWT 协议常量移动到用户服务 controller、repository 或全局 constants 包

#### Scenario: Token TTL fallbacks are owned by auth service
- **WHEN** 用户服务认证流程在配置缺失或 TTL 为零值时选择 access token、refresh token 或 password-change token fallback TTL
- **THEN** fallback 常量 MUST 位于认证 service 边界附近
- **THEN** 示例 YAML 和 DTO example 如与 fallback 不一致，MUST 明确其是部署示例、响应示例或安全 fallback

#### Scenario: Session key formats stay with Redis session repository
- **WHEN** 实现整理 Redis 中 token version、session 或用户 session index key 格式
- **THEN** key format 常量 MUST 位于 Redis auth session repository 边界附近
- **THEN** 实现 MUST NOT 将 Redis key format 暴露为无业务 owner 的全局常量
