## Context

用户会话控制能力当前通过 Redis 保存 Refresh Token 会话、用户活跃会话索引和 `token_version` 缓存，并通过 PostgreSQL 中的用户 `token_version` 作为真实版本来源。现有 Redis auth session repository 在 `GetCurrentTokenVersion` 缓存未命中时直接解析用户 UUID、调用 `UserTokenVersionRepository.GetTokenVersion` 回源 PostgreSQL，并回填 Redis 缓存。

这种实现把 Redis 存储适配、缓存策略、用户数据读取和认证校验边界放在同一个 repository 中。虽然当前依赖已收窄为 `UserTokenVersionRepository`，但 Redis repository 仍不只是 Redis 适配层；后续修改 token version 校验策略、错误映射、缓存回填策略或替换缓存实现时，容易牵连 Redis repository 和用户 repository 组合方式。

## Goals / Non-Goals

**Goals:**

- 保持用户会话控制对外行为不变：登录、刷新、改密、退出当前设备和退出全部设备的 API、错误语义和响应信封兼容。
- 将 token version 缓存读写与 PostgreSQL 回源读取策略解耦。
- 让 Redis auth session repository 只负责 Redis 会话记录、用户会话索引、token version cache key 的读写和删除。
- 将“cache miss 后回源 PostgreSQL 并回填 Redis”的策略放到 service 层认证会话组件或专门的 resolver 中。
- 保持 Fx 注入仍使用既有 `cache_redis`、`user_db` 和 `auth.token_version_cache_ttl` 配置。

**Non-Goals:**

- 不新增 HTTP API、不修改 DTO、不改变错误码或响应结构。
- 不修改 Ent schema、Atlas migration 或数据库表结构。
- 不引入新的外部缓存、消息队列或分布式锁依赖。
- 不改变 `token_version` 作为 PostgreSQL 真实来源、Redis 作为缓存的长期语义。

## Decisions

1. Redis repository 拆出 cache-only token version 能力。

   `repository.AuthSessionRepository` 不再暴露“获取当前真实 token version”的语义，而是保留或拆分为更精确的 Redis 操作，例如读取缓存、写入缓存和删除缓存。Redis 实现只处理 Redis key、TTL、parse/format 和 session/index 操作，不直接调用用户 repository。

   备选方案是保留当前接口并仅把字段名从 `repo` 改为更小接口；该方案改动最小，但无法解决 Redis repository 承担 DB 回源策略的问题。

2. 在认证会话 service 组件组合 cache 与 DB reader。

   `authSessionManager` 已经同时持有 `UserTokenVersionRepository` 和 `AuthSessionRepository`，适合承载 token version 校验编排。实现时可在该组件内新增私有方法，或新增未导出的 resolver 类型，通过 Redis cache 读取、cache miss 回源 `UserTokenVersionRepository.GetTokenVersion`、成功后回填 cache。这样 controller 仍只做 HTTP 解析，service 负责业务编排，repository 分别负责 Redis 或 PostgreSQL 数据访问。

   备选方案是在 middleware 中组合 cache 与 DB reader。该方案可以靠近 Access Token 校验，但当前 Refresh Token 和 password-change token 校验主要位于 auth service，会导致校验逻辑分散，不作为首选。

3. 保持缓存错误处理语义与现有行为兼容。

   Redis 非 miss 读取错误继续作为内部错误返回；Redis 缓存值格式非法或非正数应被视为无效缓存并触发 DB 回源；DB 回源成功后按照 `auth.token_version_cache_ttl` 回填，TTL 非正数时继续使用既有默认值。DB 返回用户不存在时继续由 service 映射为现有 not found 或 token invalid 语义。

   备选方案是缓存回填失败时忽略错误以提升可用性；这会改变现有失败语义，可能掩盖 Redis 写入问题，本次不改变。

4. Fx 依赖图保持稳定。

   Redis auth session repository 的 Fx params 不再注入用户 token version repository。Auth service 或 auth session manager 继续通过已有 `AuthServiceParams.TokenVersions` 获取 PostgreSQL token version reader/writer，并通过 `AuthServiceParams.Sessions` 获取 Redis session/cache store。

## Risks / Trade-offs

- [Risk] 接口拆分会影响现有测试 stub 和 Redis repository 单测。→ Mitigation：同步更新 service 层和 Redis 层单测，分别覆盖 cache hit、cache miss 回源、非法缓存回源、Redis 错误、DB not found 和回填 TTL。
- [Risk] 如果抽象命名过泛，后续仍可能把业务策略塞回 repository。→ Mitigation：接口命名使用 cache/session 语义，避免 `GetCurrentTokenVersion` 这种真实来源语义出现在 Redis repository 接口上。
- [Risk] cache miss 回源逻辑移动后可能改变错误映射位置。→ Mitigation：保持 `authSessionManager` 现有公开方法返回路径不变，并复用当前 domain error 到 response error 的映射。

## Migration Plan

- 重构内部接口和 Fx 注入，不需要数据库 migration 或数据迁移。
- 部署时可直接替换服务二进制；Redis 现有 session key、token version cache key 和 TTL 语义保持兼容。
- 回滚时使用上一版本服务即可；本变更不改变持久化数据格式。

## Open Questions

- 无待决问题；实现时在 service 内私有 resolver 和独立未导出类型之间选择最小改动方案。
