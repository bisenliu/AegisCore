## Why

当前用户会话索引使用 Redis Set 加 `Expire` 维护活跃 `session_id`，无法按单个会话过期时间清理成员，长期低频访问用户可能残留已过期的冷成员，影响退出全部设备等路径的扫描成本。鉴权热路径中的 `parseTokenVersion` 使用 `fmt.Sscan` 解析整数，也会在高并发 token version 校验中产生不必要的 CPU 开销。

## What Changes

- 将 `user-session-control` 中 Redis 用户活跃会话索引从 Set 改为 ZSet，Key 保持 `auth:user:<user_id>:sessions` 不变，member 为 `session_id`，score 为会话过期时间 Unix 时间戳。
- 在创建会话、读取会话索引、退出当前设备、退出全部设备等涉及用户会话索引的操作中，使用当前 Unix 时间戳执行 `ZRemRangeByScore` 清理已过期成员。
- 删除依赖用户会话索引 Key 级 `Expire` 作为主要清理手段的设计，改为基于每个会话真实过期时间清理索引成员。
- 将 `parseTokenVersion` 从 `fmt.Sscan` 改为 `strconv.ParseInt(value, 10, 64)`，降低鉴权高并发热路径解析开销。
- 不支持旧 Redis Set 数据兼容、类型判断或 `WRONGTYPE` 降级处理；该系统按全新上线处理。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `user-session-control`: 用户活跃会话索引的 Redis 数据结构和清理语义从 Set + Key TTL 调整为 ZSet + per-session expiration score，并要求索引访问路径主动清理过期成员。

## Impact

- 影响代码：`user-services/internal/service/session_store.go`，重点包括 `CreateSession`、`DeleteSession`、`DeleteAllUserSessions`、会话索引读取逻辑、`parseTokenVersion` 和 imports。
- 影响 Redis 数据：`auth:user:<user_id>:sessions` 的类型变为 ZSet；member 保持 `session_id`，score 为 `Session.ExpiresAt.Unix()` 或等价的会话过期 Unix 秒。
- 影响 API：无新增或变更 HTTP 路由、请求体、响应信封或错误码。
- 兼容性：全新上线系统不做旧 Set 数据迁移或兼容；部署前 Redis 中不应保留同名 Set 数据。
- 依赖：继续使用现有 `github.com/redis/go-redis/v9`，无需新增第三方依赖。
