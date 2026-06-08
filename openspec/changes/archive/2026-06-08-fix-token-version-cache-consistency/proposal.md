## Why

当前 `user-session-control` 在改密和退出全部设备时先递增 PostgreSQL `token_version`，再删除 Redis token version cache；认证校验在 Redis cache 命中时直接信任缓存值。该顺序会在并发请求或 Redis 删除失败时允许旧 Access Token 在缓存 TTL 内继续通过认证，违背“用户级安全事件后旧 token 应失效”的安全预期。

## What Changes

- 将用户级安全事件后的 Redis token version cache 处理从“删除旧缓存”调整为“写入或刷新为 PostgreSQL 返回的新版本”，让后续认证即使命中 Redis 也会看到新版本。
- 明确改密和退出全部设备成功后，旧 Access Token 与旧 Refresh Token 必须因 token version 不一致或会话失效被拒绝。
- 补充 Redis token version cache 写入失败时的错误处理要求：不得在旧缓存仍可能被信任的情况下报告吊销成功。
- 保持现有 HTTP 路由、请求/响应结构、错误码、配置项和数据库 schema 不变。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `user-session-control`: 修改用户级 token/session 失效时 Redis token version cache 的一致性要求，避免旧版本 cache 命中导致旧 token 在 TTL 窗口内继续通过认证。

## Impact

- 影响代码：`user-services/internal/service/auth_sessions.go`、`user-services/internal/repository/auth_session_repository.go`、`user-services/internal/repository/redis/auth_session_repository.go` 及相关测试。
- 影响认证行为：改密和退出全部设备成功后，旧 Access Token 应立即因版本不一致被拒绝；旧 Refresh Token 应因版本不一致或 Redis 会话删除被拒绝。
- API 兼容性：不新增或修改 HTTP 路由、请求体、响应信封、错误码或配置键。
- 数据兼容性：不修改 Ent schema、PostgreSQL migration 或 Redis key 命名格式。
