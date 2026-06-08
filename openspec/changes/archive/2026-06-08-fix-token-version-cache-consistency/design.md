## Context

`user-session-control` 通过 PostgreSQL `token_version` 和 Redis token version cache 共同校验 Access Token、Refresh Token 和受限改密凭据。当前 `authSessionManager.currentTokenVersion` 在 Redis cache 命中时直接返回缓存值，不回源 PostgreSQL；`RevokeAllUserSessions` 在用户级安全事件中先递增 PostgreSQL `token_version`，再删除 Redis token version cache 和 Refresh Token 会话记录。

该顺序在 Redis 删除成功后的常规路径可以让后续请求回源 PostgreSQL，但在 DB 更新与 Redis 删除之间存在并发窗口；如果 Redis 删除失败，旧版本缓存会继续被认证中间件信任直到 TTL 到期。受影响的包和分层边界如下：

- `common/http/middleware`: 认证中间件只调用 `TokenVersionValidator`，不直接接触 Redis 或 PostgreSQL。
- `user-services/internal/service`: 认证会话组件编排 DB token version 递增、Redis token version cache 刷新和 Refresh Token 会话删除。
- `user-services/internal/repository`: repository 抽象定义 Redis 会话和 token version cache 操作契约。
- `user-services/internal/repository/redis`: Redis 实现负责读写 token version cache、Refresh Token 会话和用户活跃会话索引。

## Goals / Non-Goals

**Goals:**

- 改密或退出全部设备成功后，旧 Access Token 不应因为 Redis 旧版本缓存命中而继续通过认证。
- 复用 PostgreSQL `IncrementTokenVersion` 返回的新版本，将 Redis token version cache 覆盖为新版本。
- Redis token version cache 刷新失败时，不报告用户级会话吊销成功，避免调用方误以为旧 token 已失效。
- 保持现有 controller/service/repository 分层和 Fx 依赖注入边界。
- 补充 service 层和 Redis repository 层测试，覆盖旧缓存被新版本覆盖、刷新失败、Refresh Token 会话删除失败等路径。

**Non-Goals:**

- 不新增 HTTP API、请求字段、响应字段或错误码。
- 不修改 JWT claims 结构、Redis key 命名格式或配置项名称。
- 不修改 Ent schema、PostgreSQL migration 或 Atlas 配置。
- 不引入分布式锁、Redis Lua 脚本、CDC 或跨存储强事务。

## Decisions

1. 在 DB 递增后刷新 Redis token version cache 为新版本。

   认证路径 cache hit 会直接信任 Redis 值，因此删除旧缓存无法消除 DB 更新到 Redis 删除之间的并发窗口。覆盖写入新版本后，并发认证即使命中 Redis，也会看到新版本并拒绝旧 token。备选方案是删除缓存后强制回源 DB，但这仍依赖删除成功，且无法改善删除前窗口。

2. 将 repository 抽象从“失效 token version”表达调整为“保存当前 token version”。

   现有 `CacheTokenVersion(ctx, userID, tokenVersion)` 已提供覆盖写能力，service 层可以直接复用它。为了让接口语义更明确，`RevokeAllUserSessions` 应调用写入新版本的方法，而不是调用仅 `DEL` 的 `InvalidateUserTokenVersion`。如需保留删除方法，应仅作为低层缓存维护能力，不作为安全事件成功路径。

3. Redis token version cache 写入失败时中止吊销流程并返回错误。

   如果写入新版本失败且旧缓存仍存在，继续删除 Refresh Token 会话并返回成功会造成 Access Token 仍可能通过认证。返回错误能保持“成功响应意味着旧 token 已不可被缓存旧值放行”的语义。备选方案是先删除旧缓存再写新缓存，但在删除失败场景仍无法保证旧缓存不被信任。

4. Refresh Token 会话删除仍在 token version cache 刷新之后执行。

   新版本 cache 刷新成功后，旧 Access Token 和旧 Refresh Token 都会因版本不一致被拒绝；随后删除 Redis Refresh Token 会话记录可以减少残留会话。如果会话删除失败，接口仍应返回错误以维持现有错误处理语义，但旧 token 不应因为会话残留而通过版本校验。

## Risks / Trade-offs

- Redis 写入新版本失败后 DB 已递增，调用方收到失败但旧 token 可能仍受旧缓存影响。Mitigation: 不报告成功，并记录错误；后续可通过重试或运维处理恢复，且 TTL 仍限制最终窗口。
- DB 与 Redis 仍不是跨存储原子事务。Mitigation: 本变更把成功路径从“删除旧值后回源”改为“覆盖新值”，收窄 cache-hit 放行窗口，但不声称提供跨存储强事务。
- Redis cache TTL 仍适用于新版本缓存。Mitigation: 保持 `auth.token_version_cache_ttl` 行为不变；新缓存过期后会按现有逻辑回源 PostgreSQL 并回填。
- 接口方法语义调整会影响测试 stub。Mitigation: 同步更新 service、bootstrap 和 Redis repository 测试中的 stub 与断言。

## Migration Plan

该变更不涉及数据迁移或 Ent 生成。部署新版本后，后续改密和退出全部设备操作会写入新 token version cache。已有 Redis 旧缓存会按原 TTL 过期，或在用户下一次安全事件中被新版本覆盖。

回滚到旧版本不需要数据回滚；Redis 中保存的新版本 cache 与 PostgreSQL 当前 `token_version` 一致，旧版本代码仍可读取。

## Open Questions

无。
