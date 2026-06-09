## Why

当前认证会话 Redis 实现仍使用以 `session_id` 为主体的旧 key 结构，并且 `RotateSession` 依赖 WATCH/MULTI 乐观事务；在同一 Refresh Token 被高并发刷新时，WATCH 冲突会放大为重试风暴，同时旧 key 无法按用户维度直接定位会话记录。`user-session-control` 需要直接切换到按用户聚合的新 key 格式，并用 Redis Lua 脚本把会话轮换和批量撤销变成单次原子操作。

## What Changes

- **BREAKING**: Redis 认证会话 key 直接切换为新格式，不保留旧 key 兼容读取、双写或迁移期分支。
- **BREAKING**: Refresh Token 会话记录 key 从 `<prefix>:auth:session:<sessionID>` 调整为 `<prefix>:auth:session:{<userID>}:<sessionID>`，用户会话索引调整为 `<prefix>:auth:user:sessions:{<userID>}`，token version 缓存调整为 `<prefix>:auth:user:token_version:{<userID>}`；`<prefix>` 继续沿用原有 `config.App.Name` 命名空间规则，空 app name 时省略。
- 将 `RotateSession` 的 WATCH/MULTI 实现替换为 Redis Lua 脚本，脚本内完成旧会话读取与匹配校验、新会话写入、用户索引更新、旧会话删除和过期索引清理。
- 将 `DeleteAllUserSessions` 改为 Redis Lua 脚本，并使用 `UNLINK` 异步释放用户全部会话 key 和索引 key，避免批量删除阻塞 Redis 主线程。
- 更新 Redis key builder、session store 单元测试和并发轮换测试，确保所有读写路径只使用新 key 格式。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 修改 Redis 认证会话存储契约，要求新 key 格式、Lua 原子轮换，以及基于 Lua + UNLINK 的用户会话批量撤销。

## Impact

- 主要影响代码：`user-services/internal/features/auth/domain/rediskeys.go`、`user-services/internal/features/auth/infra/redis/session_store.go` 及其测试。
- 影响认证流程：登录、刷新、退出当前设备、退出全部设备、token version 校验的 Redis key 命名与会话删除行为。
- 外部 HTTP API、请求/响应 DTO、统一响应信封、JWT claims、PostgreSQL schema 和 Atlas migration 不变。
- Redis 数据兼容性：已有旧格式 Redis 会话和 token version 缓存不会被读取，也不会迁移；部署后用户需要重新登录或通过正常认证流程重建新格式会话。
