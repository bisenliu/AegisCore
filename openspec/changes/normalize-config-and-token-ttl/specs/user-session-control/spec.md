## ADDED Requirements

### Requirement: Centralize default authentication TTL values

认证会话服务 SHALL 将默认 Access Token TTL、Refresh Token TTL 和 token version cache TTL 集中声明为包级常量。实现 MUST 使用这些常量作为零值或非法 TTL 配置的兜底值，并保持现有默认生命周期不变。

#### Scenario: Default access token TTL uses named constant
- **GIVEN** 认证配置未提供有效 `auth.jwt.access_token_ttl`
- **WHEN** 登录或刷新流程签发 Access Token
- **THEN** 系统 MUST 使用集中声明的默认 Access Token TTL 常量
- **THEN** 默认 Access Token TTL MUST 保持为 15 分钟

#### Scenario: Default refresh token TTL uses named constant
- **GIVEN** 认证配置未提供有效 `auth.jwt.refresh_token_ttl`
- **WHEN** 登录或启用轮转的刷新流程创建 Refresh Token 会话
- **THEN** 系统 MUST 使用集中声明的默认 Refresh Token TTL 常量
- **THEN** 默认 Refresh Token TTL MUST 保持为 7 天

#### Scenario: Default token version cache TTL uses named constant
- **GIVEN** 认证配置未提供有效 `auth.token_version_cache_ttl`
- **WHEN** session store 回源 PostgreSQL 并写入 Redis token version 缓存
- **THEN** 系统 MUST 使用集中声明的默认 token version cache TTL 常量
- **THEN** 默认 token version cache TTL MUST 保持为 5 分钟

#### Scenario: Explicit TTL config still takes precedence
- **GIVEN** 认证配置提供有效 Access Token TTL、Refresh Token TTL 或 token version cache TTL
- **WHEN** 认证服务签发 token 或 session store 写入缓存
- **THEN** 系统 MUST 使用显式配置值
- **THEN** 系统 MUST NOT 用默认 TTL 常量覆盖有效配置值
