## Why

当前认证会话 Redis repository 在 token version 缓存未命中时直接回源用户 PostgreSQL repository，导致 Redis 存储适配、缓存策略和用户数据读取边界混在一起。该实现可以工作，但会增加后续调整认证校验、缓存 TTL、DB 回源错误映射或替换缓存实现时的耦合成本。

## What Changes

- 将 token version 缓存读写与用户 token version 数据读取策略解耦。
- 让 Redis auth session repository 专注 Redis session、Redis key 和 token version cache 操作，不再直接调用用户 repository 回源 PostgreSQL。
- 在 service 层或专门的 token version resolver 中组合 Redis cache 与 `UserTokenVersionRepository`，保持“cache miss 回源 PostgreSQL 并回填 Redis”的行为。
- 保持登录、刷新、改密、退出当前设备、退出全部设备的外部 API、响应信封、错误码、配置字段和数据模型兼容。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 补充认证会话组件、Redis session repository、token version cache 和 PostgreSQL token version reader 的职责边界要求。

## Impact

- 影响代码：`user-services/internal/repository/auth_session_repository.go`、`user-services/internal/repository/redis/auth_session_repository.go`、`user-services/internal/service/auth_sessions.go`、`user-services/internal/service/auth_service.go` 及相关测试。
- API 影响：无；HTTP 路由、请求/响应结构、错误码和认证语义保持不变。
- 配置影响：无；继续使用 `auth.token_version_cache_ttl` 和既有 `cache_redis`、`user_db` 运行时依赖。
- 数据模型影响：无；不修改 Ent schema 或 Atlas migration。
