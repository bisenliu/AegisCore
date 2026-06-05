## Why

当前 Redis 认证会话创建流程同时依赖传入 `ttl` 和 `session.ExpiresAt` 表达过期时间，导致 session key 的实际 TTL 与用户会话 ZSet score 可能不一致。用户会话 ZSet 自身没有 TTL，若索引 score 或清理策略与真实 session key 生命周期脱节，过期 `session_id` 可能长期残留并逐步增加存储和操作成本。

## What Changes

- 统一 Refresh Token 会话过期时间的单一来源，避免 `ExpiresAt` 与 `ttl` 双重来源导致 Redis session key 和用户会话索引不一致。
- 补强用户活跃会话 ZSet 的过期或清理策略，确保过期 member 不会长期残留。
- 明确 `DeleteAllUserSessions` 等批量操作只应处理仍有效或已清理后的索引数据，降低遍历脏数据的风险。
- 评估并在需要时采用 Lua 脚本或更严格的 Redis 事务封装，使 session key 与用户会话索引写入具备更强一致性。
- 不改变现有认证 HTTP API、响应信封、Redis key 命名格式、token claims 或 PostgreSQL 数据模型。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 修改 Redis Refresh Token 会话记录和用户活跃会话索引的过期一致性、索引清理和批量删除行为要求。

## Impact

- 影响代码：`user-services/internal/repository/redis/auth_session_repository.go` 及相关认证会话 repository 测试。
- 影响系统：Redis 中 `auth:session:<session_id>` session key 与 `auth:user:<user_id>:sessions` ZSet 索引的生命周期一致性。
- 兼容性：不改变公开 HTTP API、错误码、响应格式、Redis key 命名和 PostgreSQL schema；现有脏索引数据需要通过新增清理策略在后续写入、删除或批量操作中逐步清理。
