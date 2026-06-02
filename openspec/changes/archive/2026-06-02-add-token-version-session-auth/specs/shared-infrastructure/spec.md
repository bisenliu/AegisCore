## ADDED Requirements

### Requirement: Load refresh token authentication configuration
系统 SHALL 从 YAML 配置和 `AEGISCORE_` 环境变量覆盖中加载 Refresh Token 和认证会话相关配置，包括 Refresh Token TTL、token version 缓存 TTL 和 Refresh Token 轮转开关。配置加载器 MUST 只负责读取、覆盖和反序列化这些字段，不得在 `common/config.Load` 阶段执行 required、字段存在性或基础取值范围校验。

#### Scenario: Load refresh token config from YAML
- **Given** YAML 配置包含 `auth.jwt.refresh_token_ttl`、`auth.token_version_cache_ttl` 和 `auth.refresh_token_rotation`
- **When** `common/config.Load` 被调用
- **Then** 系统 MUST 将这些字段反序列化到 `config.Config` 的认证配置中
- **Then** 配置加载器 MUST NOT 因 TTL 为零值或轮转开关未显式设置而返回校验错误

#### Scenario: Override refresh token config from environment
- **Given** YAML 配置包含 Refresh Token 相关认证配置
- **Given** 环境变量提供 `AEGISCORE_AUTH_JWT_REFRESH_TOKEN_TTL` 或 `AEGISCORE_AUTH_TOKEN_VERSION_CACHE_TTL`
- **When** `common/config.Load` 被调用
- **Then** 系统 MUST 使用环境变量覆盖后的认证配置值

### Requirement: Reuse user service datastore dependencies for authentication sessions
系统 SHALL 复用用户服务已声明的 `cache_redis` Redis client 和 `user_db` Ent client 支撑认证会话能力。认证会话能力 MUST NOT 因配置中存在其他 Redis 或 PostgreSQL 命名实例而自动连接未声明的实例。

#### Scenario: Authentication session store uses cache redis
- **Given** 用户服务运行时声明 `cache_redis` Redis client
- **When** 认证会话 store 被 Fx 构造
- **Then** 该 store MUST 使用具名 `cache_redis` Redis client
- **Then** 系统 MUST NOT 为认证会话额外连接未声明 Redis 实例

#### Scenario: Token version lookup uses user database
- **Given** 用户服务运行时声明具名 `user_db` Ent client
- **When** token version validator 需要回源 PostgreSQL
- **Then** 系统 MUST 使用 `user_db` Ent client 查询用户当前 `token_version`
- **Then** 系统 MUST NOT 连接未声明 PostgreSQL 实例
