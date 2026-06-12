# Design

## Overview

本变更采用“安全语义同步、物理垃圾清理异步”的边界：

```text
登录/refresh rotation session 上限裁剪
  -> 同步执行
  -> Redis Lua 原子完成
  -> 影响 token 是否可继续使用

logout-all / password change / admin revoke 的批量旧 key 删除
  -> 同步完成 token_version 失效
  -> workerpool 异步删除 Redis session key
  -> 不影响旧 token 立即失效
```

同一用户允许多个活跃 refresh session，但数量受配置限制。默认上限为 5。超过上限时裁剪 sorted-set 中最旧的 session，并删除对应 session payload key。

## Configuration

新增配置：

```yaml
auth:
  # 同一用户最多保留的活跃 Refresh Token 会话数；0 表示不限制。
  max_active_sessions_per_user: 5
```

建议 Go 配置字段：

```go
type AuthConfig struct {
    JWT                      JWTConfig     `mapstructure:"jwt"`
    TokenVersionCacheTTL     time.Duration `mapstructure:"token_version_cache_ttl"`
    RefreshTokenRotation     bool          `mapstructure:"refresh_token_rotation"`
    MaxActiveSessionsPerUser int           `mapstructure:"max_active_sessions_per_user"`
}
```

校验语义：

- `> 0`：启用上限裁剪。
- `0`：不限制，用于本地调试或兼容旧行为。
- `< 0`：非法配置，启动校验失败。

默认值建议在 `user-service/configs/config.yaml` 显式写入 `5`。如果配置 loader 有默认填充机制，也应保持 YAML 示例与默认行为一致。

环境变量覆盖沿用现有 `AEGISCORE_` 机制，例如：

```text
AEGISCORE_AUTH_MAX_ACTIVE_SESSIONS_PER_USER=5
```

## Application Boundary

session 上限是 auth feature 的业务安全策略，应由 auth application 层持有，不应让 Redis adapter 私自从配置中决定业务规则。

`UseCaseDeps` 从 `params.Config.Auth.MaxActiveSessionsPerUser` 读取上限值，并由 auth session lifecycle 保存为 `maxActiveSessionsPerUser int`。当前策略只有一个数值，port 直接传入该 int，避免为单字段引入额外 wrapper struct。

推荐 port 调整：

```go
type AuthSessionStore interface {
    CreateSession(ctx context.Context, session authdomain.AuthSession, ttl time.Duration, maxActiveSessionsPerUser int) error
    RotateSession(ctx context.Context, oldSession authdomain.AuthSession, newSession authdomain.AuthSession, ttl time.Duration, maxActiveSessionsPerUser int) error
    GetSession(ctx context.Context, userID string, sessionID string) (authdomain.AuthSession, error)
    DeleteSession(ctx context.Context, userID string, sessionID string) error
    DeleteAllUserSessions(ctx context.Context, userID string) error
    GetCachedTokenVersion(ctx context.Context, userID string) (int64, error)
    CacheTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
    DeleteCachedTokenVersion(ctx context.Context, userID string) error
}
```

不要把 `config.Config`、Redis client、workerpool 或 Lua 细节传入 application use case。

## Redis Data Model

沿用当前 Redis key schema：

```text
auth:session:{user_id}:<session_id>
auth:user:sessions:{user_id}
```

session payload key 存储 refresh session JSON，TTL 等于 refresh token TTL。

用户 session sorted-set：

- key：`auth:user:sessions:{user_id}`
- member：`session_id`
- score：session `expires_at` 的 Unix 秒
- TTL：最长 session TTL 加 `authSessionIndexTTLBuffer`

本变更不修改 key schema，只修改写入和轮转脚本的原子行为。

## Create Session Script

当前 `CreateSession` 使用 pipeline：

```text
SET session key
ZREMRANGEBYSCORE expired
ZADD session index
EXPIRE session index
```

建议替换为 Lua 脚本，保证“创建新 session + 裁剪旧 session”原子完成。

脚本输入建议：

```text
KEYS[1] = new session payload key
KEYS[2] = user session sorted-set key

ARGV[1] = new session payload JSON
ARGV[2] = new session payload TTL milliseconds
ARGV[3] = new session_id
ARGV[4] = new session expires_at unix seconds
ARGV[5] = now unix seconds
ARGV[6] = index TTL milliseconds
ARGV[7] = session key prefix
ARGV[8] = max active sessions per user
```

脚本流程：

```text
SET KEYS[1] ARGV[1] PX ARGV[2]
ZREMRANGEBYSCORE KEYS[2] -inf ARGV[5]
ZADD KEYS[2] ARGV[4] ARGV[3]

if max > 0 then
    overflow = ZCARD(KEYS[2]) - max
    if overflow > 0 then
        stale = ZRANGE KEYS[2] 0 overflow-1
        for each session_id in stale:
            DEL session_prefix .. session_id
        ZREM KEYS[2] stale...
    end
end

if PTTL(KEYS[2]) < index_ttl then
    PEXPIRE KEYS[2] index_ttl
end
```

注意事项：

- `max <= 0` 时不执行上限裁剪，但仍执行过期索引懒清理。
- 如果新 session 被算入最旧集合，理论上只有 `max == 0` 或极端时间 score 竞争才可能出问题；实现时应保证启用上限时 `max >= 1`，新 session score 使用未来过期时间，旧 session 更早过期，应裁剪旧项。
- session key prefix 应由 `RedisKeyBuilder.AuthSessionPrefix(userID)` 生成，不在 Lua 内拼接业务前缀。

## Rotate Session Script

当前 `RotateSession` 已经通过 Lua 原子完成：

```text
读取旧 session
校验 old payload 与 claims 匹配
SET 新 session
清理过期索引
ZADD 新 session
ZREM 旧 session
DEL 旧 session key
刷新 index TTL
```

建议在该脚本尾部加入同样的上限裁剪逻辑：

```text
if max > 0 then
    overflow = ZCARD(user sessions) - max
    if overflow > 0 then
        stale = ZRANGE user sessions 0 overflow-1
        for each session_id in stale:
            DEL session_prefix .. session_id
        ZREM user sessions stale...
    end
end
```

这能让历史已有 14 个 session 的用户在 refresh 或登录后逐步收敛到上限。

`RotateSession` 的既有错误语义保持不变：

- 旧 session 不存在：`ErrAuthSessionNotFound`
- 旧 session payload 与 claims 不匹配：`ErrAuthSessionMismatch`
- 脚本异常：包装为 infrastructure error

## Workerpool Boundary

不要用 workerpool 做登录上限裁剪。

原因：

- 上限裁剪影响 refresh token 是否仍可使用，属于安全策略。
- 异步裁剪会产生短暂超限窗口。
- 并发登录下异步任务顺序不稳定，最终上限可能短时间失控。
- 当前 refresh 校验直接读取 session key；只要旧 key 还在，旧 refresh token 就仍可用。

workerpool 只继续用于批量物理删除：

- `logout-all`
- 强制改密后的全部 session 撤销
- 未来管理员封禁或安全事件导致的全量撤销

这类场景应先同步递增或刷新 token version，使旧 access/refresh token 立即失效；旧 Redis session key 是否稍后删除只影响内存回收和可观测清理，不影响安全结果。

当前 `auth_session_purge_pool` 可以继续复用，无需新增 worker pool。

## Failure Semantics

登录创建 session：

- JWT 签发成功但 Redis `CreateSession` 失败：返回错误，不返回 token 响应。当前 `issueTokenPair` 已符合这个行为。
- Redis Lua 创建成功但裁剪失败：整个脚本失败，返回错误。Redis Lua 单脚本执行不会提交半截 Lua 逻辑中的显式回滚，但脚本运行时错误需要通过参数和命令使用避免；不要在裁剪循环中使用可能触发类型错误的动态命令。
- 裁剪删除旧 session 成功：旧 refresh token 后续校验失败。

refresh rotation：

- 旧 session 校验失败：返回 token invalid。
- 新 token 签发成功但 Redis rotation 失败：返回错误，不返回新 token 响应。
- Redis rotation 成功：旧 refresh token 立即失效，新 refresh token 可用，总 session 数不超过上限。

配置：

- `max_active_sessions_per_user < 0`：启动校验失败。
- `max_active_sessions_per_user == 0`：保留旧行为，不限制数量。

## Logging

日志消息仍使用英文，字段使用稳定英文 snake_case。

建议在成功裁剪时记录 `Info` 或 `Debug`，避免高频登录造成日志噪音。第一版可以不为每次裁剪新增成功日志，只在已有 `auth session created` 和 `auth session rotated` 日志中追加字段：

- `max_active_sessions_per_user`
- `pruned_sessions`

如果 Lua 脚本返回被裁剪数量，可用于日志和测试断言；如果不返回，也可只通过 Redis 状态测试，不扩大日志字段。

错误日志继续由 application lifecycle 包装：

- `create auth session failed`
- `rotate auth session failed`

不要记录 token、Authorization header、Cookie、password 或原始请求体。

## Documentation Updates

需要更新：

- `docs/ARCHITECTURE.md` Infrastructure 或 Current Constraints：说明 auth Redis session 支持每用户活跃上限，上限裁剪同步执行。
- `docs/ARCHITECTURE.md` Common Organization：如果提及 workerpool，补充它不负责 session 上限裁剪。
- `AGENTS.md` Repository Shape 或 Current Feature Areas：补充认证会话控制包含受控多会话治理。
- `AGENTS.md` Repository Rules：补充影响 token 有效性的 session 策略必须同步落在 application + infrastructure adapter 边界，批量物理清理可使用 workerpool。

## Rollout

该变更会主动删除超出上限的旧 refresh session。上线后：

1. 存量超过 5 个 session 的用户不会立刻全部收敛，直到下次登录或 refresh rotation 触发写路径。
2. 如果希望上线时立即收敛，可单独运行 Redis 运维脚本，但本变更不包含该脚本。
3. 用户被裁剪的旧设备 refresh token 会失效，旧 access token 在 access TTL 内仍可能可用，直到过期或 token version 撤销。若需要立即让被裁剪设备 access token 也失效，需要额外设计 per-session access token 校验或 token denylist，本变更不包含。

最后一点是有意边界：当前系统的 access token 只校验 token version，不查 session key。裁剪 refresh session 主要控制长期续期能力，不等于立即踢掉仍未过期的 access token。

## Tests

配置测试：

- 默认 YAML 可加载 `max_active_sessions_per_user: 5`。
- 环境变量可覆盖该值。
- `0` 通过校验。
- 负数校验失败。

Redis adapter 测试：

- `CreateSession` 连续创建 6 个 session，上限 5，最终只保留最近 5 个。
- 被裁剪 session `GetSession` 返回 `ErrAuthSessionNotFound`。
- 用户 session sorted-set `ZCARD` 不超过 5。
- 上限为 0 时连续创建 session 不裁剪。
- 已过期索引项仍会被懒清理。
- 并发创建 session 最终不超过上限。
- `RotateSession` 删除旧 session、创建新 session，并执行上限裁剪。
- `RotateSession` 的 not found/mismatch 语义不变。

Application command 测试：

- `Login` 或 `issueTokenPair` 会把配置中的 session 上限传给 session lifecycle/store。
- refresh rotation 会把相同策略传入 `RotateTokenSession`。
- Redis create/rotate 失败时不返回 token 响应。

回归测试：

- `DeleteSession` 行为不变。
- `DeleteAllUserSessions` 仍使用 `auth_session_purge_pool` 批量清理。
- `RevokeAllUserSessions` token version 语义不变。
