# Add auth token version local cache

## What

为认证中间件使用的 token version 校验增加进程内短 TTL 本地缓存，并使用 `singleflight` 合并同一实例内同一用户的并发回源。

当前受保护接口请求链路为：

```text
AuthWithTokenVersionValidator
  -> tokenVersionValidator.ValidateTokenVersion
  -> sessions.GetCachedTokenVersion
  -> Redis GET token_version
```

本变更在 `common/runtime/localcache` 中提供无业务语义的进程内短 TTL 缓存 primitive，并在 `ValidateTokenVersion` 所在的 auth application 组件中使用该缓存作为 L1 本地缓存。这样同一服务实例内、同一用户在短时间内的连续请求会优先命中内存，只有本地 miss 时才访问 Redis；Redis miss 时仍按现有逻辑回源 PostgreSQL 并回填 Redis。

本地缓存只缓存“用户当前 token_version”，不缓存 JWT parse 结果，也不改变 access token、refresh token 或 password_change token 的过期校验。

## Why

当前 Redis token version 投影只能避免每次校验都访问 PostgreSQL，但缓存命中时仍然需要一次 Redis 网络往返。在单实例高 QPS 的受保护 API 下，token version 校验会形成稳定的 Redis GET 压力。

典型高频场景包括：

- 前端首屏同时请求当前用户、权限、角色、通知和配置等多个接口。
- 同一用户打开多个浏览器 tab 或多设备同时在线。
- 移动端弱网重试导致同一 access token 短时间重复请求。
- 内部系统使用同一管理员 token 执行批量操作。

这些请求在同一服务实例上会使用相同 user ID 和 token version。短 TTL 本地缓存可以减少 Redis RTT 和 Redis QPS；`singleflight` 可以避免本地 miss 或 Redis key 过期时，同一实例内并发请求同时回源 Redis/PostgreSQL。

## Confirmed Findings

已确认真实存在的问题：

- [common/http/middleware/auth.go](/Users/liubisen/Desktop/sander/Project/my/AegisCore/common/http/middleware/auth.go:83) 在每个受保护请求上调用 `ValidateTokenVersion`。
- [user-service/internal/features/auth/application/validators/token_version_validator.go](/Users/liubisen/Desktop/sander/Project/my/AegisCore/user-service/internal/features/auth/application/validators/token_version_validator.go:37) 当前固定先查 Redis，再在 miss 时回源数据库。
- [user-service/internal/features/auth/infrastructure/redis/session_store.go](/Users/liubisen/Desktop/sander/Project/my/AegisCore/user-service/internal/features/auth/infrastructure/redis/session_store.go:181) 当前 Redis 命中路径是直接 `GET`。
- 当前默认 `auth.token_version_cache_ttl` 是 `30s`，不是旧设计中的 `5m`；本变更不再调整 Redis TTL。

已确认暂不作为本变更实现范围的问题：

- `DeleteAllUserSessions` 当前没有使用 Redis `SCAN`，也没有在退出全部设备主路径直接执行 `ZREMRANGEBYSCORE`；它先 `RENAME` 用户会话索引到 purge key，再后台按批次 `ZRANGE`、`UNLINK`、`ZREM`。
- 默认 `max_active_sessions_per_user: 5` 下，单用户 session ZSET 通常很小，Redis 大 key 清理不是当前最确定的热点。
- Redis session 清理风险先通过观测、阈值和 slowlog 验证，不与 token version 热点优化混在同一实现中。

## Scope

包括：

- 在 `common/runtime/localcache` 中新增通用短 TTL 本地缓存 primitive，公开 `New`、`Get`、`Set`、`Delete` 方法。
- 在 auth application token version 校验组件内使用该通用 primitive 作为 L1 cache。
- 本地缓存 TTL 使用小窗口，建议默认 `1s`。
- 使用 `golang.org/x/sync/singleflight` 合并同一实例内同一 user ID 的并发加载。
- 本地缓存命中后仍调用 `common/security/auth.ValidateTokenVersion` 比较 token claims version 和当前 version。
- 本地 miss 后复用现有 `Current(ctx, users, sessions, userID)` 逻辑，保持 Redis fallback 和 PostgreSQL 回源语义。
- 在强制改密、退出全部设备等 token version 撤销路径中，保证本实例本地缓存被失效或不产生比 TTL 更长的 stale window。
- 补充单元测试覆盖 L1 cache 命中、过期、singleflight 合并、version mismatch 和 token 已过期不受本地缓存影响的边界。

不包括：

- 不修改 JWT claim schema、token 签发、token parse 或 token TTL。
- 不修改 Redis token version key schema、Redis TTL 默认值或 Redis Lua 脚本。
- 不新增 Redis token denylist、blacklist 或单 token 撤销 API。
- 不新增 Ent schema、Atlas migration 或 PostgreSQL 表结构。
- 不修改 `DeleteAllUserSessions` 的 Redis 清理算法。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。
- 不把 token version、user ID 或 auth 业务语义放入 `common/runtime/localcache`。

## Compatibility

本变更对外部 HTTP API 兼容。

行为变化是内部性能优化：同一实例会在很短时间内复用用户当前 token version。若用户在 TTL 窗口内刚完成改密或退出全部设备，某个实例可能在本地缓存过期前继续接受旧 access token。为了控制安全窗口，默认 TTL 应保持在 `1s` 左右，并在本实例撤销路径主动删除对应 user ID 的本地缓存。

跨实例立即失效不在本变更中实现。如果后续需要严格压缩多副本撤销延迟，应单独设计基于 Redis Pub/Sub 或其他广播机制的本地 cache invalidation。

## Acceptance Criteria

- `common/runtime/localcache` 提供清晰中文注释，说明用途、参数、返回值和使用示例。
- 受保护请求的 token version 校验先查询本地 L1 cache，本地 miss 后才调用现有 Redis/PostgreSQL fallback。
- 同一实例、同一 user ID 的并发 miss 被 `singleflight` 合并。
- 本地缓存 TTL 默认为短窗口，建议 `1s`，并可在代码中清晰定位。
- 本地缓存不缓存 JWT 过期判断；过期 token 仍在 JWT parse 阶段被拒绝。
- token version mismatch 仍返回现有 `ErrTokenVersionMismatch` 语义。
- Redis 不可用、PostgreSQL 回源失败等错误语义不被本地缓存吞掉。
- 强制改密和退出全部设备等本实例内撤销路径会失效对应 user ID 的本地缓存，或有测试证明 stale window 只受短 TTL 限制。
- 相关 auth application、middleware 或 Redis adapter 测试通过。
- 不新增 `openspec/`、`docs/opsx/`、Redis denylist、Ent migration 或新的横向 shared/helper 目录。
