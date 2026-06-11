## Context

`user-session-control` 目前的撤销链路在 `user-services/internal/features/auth/app/sessions.go` 中由 `RevokeAllUserSessions` 串联完成：先调用 PostgreSQL token version store 递增 `token_version`，再写 Redis token version cache，最后删除 Redis refresh session。该链路跨 PostgreSQL 与 Redis，但没有原子提交能力；Redis 写入失败时会返回失败响应，而 PostgreSQL 中的安全版本已经前进。

强制改密流程在 `user-services/internal/features/auth/app/service.go` 中先通过凭证组件更新 `password_hash`、恢复用户状态并递增 `token_version`，随后又调用 `RevokeAllUserSessions` 再递增一次 `token_version`。这让一次改密对应两个撤销版本，既不利于审计，也扩大旧 Redis cache 与新 PostgreSQL 版本不一致的窗口。

本设计只调整认证 feature 内部 app/infra 边界和规格语义。HTTP controller、响应信封、JWT claims、配置、Ent schema、Atlas migration 和 Redis key 格式保持兼容。

## Goals / Non-Goals

**Goals:**

- 明确 PostgreSQL 用户 `token_version` 是认证撤销的唯一安全事实源。
- 让强制改密只在凭证更新持久化边界递增一次 `token_version`。
- 将 Redis token version cache 刷新和 refresh session 删除降级为可重试、幂等的投影/补偿动作。
- 让 token version 校验在 Redis cache miss、无效缓存或 Redis 读取异常时回源 PostgreSQL，避免旧 access token 被旧缓存放行至 TTL。
- 保持 `AuthService`、认证会话 lifecycle、Redis session store 和 PostgreSQL credential store 的既有分层边界。

**Non-Goals:**

- 不引入 PostgreSQL 与 Redis 的分布式事务、事务消息中间件或新外部依赖。
- 不新增认证 API、设备管理、会话列表、审计事件表或后台 worker。
- 不修改 HTTP 路由、DTO、响应 JSON 字段、错误码枚举、JWT claim 字段或 Redis key 格式。
- 不修改 Ent schema、生成代码或 Atlas migration。
- 不把用户服务特定的认证撤销编排上移到 `common`。

## Decisions

### Decision: PostgreSQL token_version 作为撤销事实源

用户级撤销的正确性以 PostgreSQL `token_version` 为准。`IncrementTokenVersion` 和 `UpdateCredentials` 成功返回后，旧 token 从安全语义上已经失效；Redis cache 和 refresh session 只是加速校验、缩短 refresh token 可用窗口的投影。

替代方案是让 `RevokeAllUserSessions` 在 Redis 写失败时尝试回滚 PostgreSQL 版本。该方案会引入更多竞态：回滚期间可能已经有新 token 或新校验读取到新版本，且无法回滚其他实例看到的状态，因此不采用。

### Decision: 改密只复用凭证更新返回的版本

`CredentialVerifier.ChangePassword` 继续通过 `UserCredentialStore.UpdateCredentials` 更新密码哈希、状态和 `token_version`，并返回 `CredentialUpdateResult.TokenVersion`。`AuthService.ChangePassword` 后续调用新的 session lifecycle 方法，例如 `RevokeUserSessionsAtVersion(ctx, userID, tokenVersion)`，只做 Redis 投影，不再调用会递增 PostgreSQL 的 `RevokeAllUserSessions`。

替代方案是从 `UpdateCredentials` 中移除 token version 递增，把改密统一走 `RevokeAllUserSessions`。该方案会把“凭证更新已提交但撤销版本尚未提交”的窗口暴露出来，且需要重新定义改密事务边界，因此不采用。

### Decision: Redis 投影方法按目标版本幂等执行

认证会话 lifecycle 增加“按指定版本撤销投影”的内部能力：先尽力刷新 Redis token version cache 为目标版本；失败时尝试删除旧 cache；随后尽力删除该用户全部 refresh session。该方法可以返回投影错误供日志和测试观察，但调用方在 PostgreSQL 版本已经提交后不得把它解释为业务状态未完成。

Redis `CacheTokenVersion` 应具备单调写入语义，可用 Lua 脚本或等价 Redis 原子操作实现：当已有缓存版本大于待写版本时，不覆盖已有版本；否则写入待写版本和 TTL。新增的 cache 驱逐方法可以按当前值条件删除，也可以直接删除 token version cache key；直接删除更简单，安全性依赖后续回源 PostgreSQL。

替代方案是继续使用普通 `SET` 覆盖缓存。该方案在并发补偿或旧请求延迟完成时可能用旧版本覆盖新版本，因此不采用。

### Decision: token version resolver 对 Redis 异常回源 PostgreSQL

`currentTokenVersion` 维持“先 Redis，后 PostgreSQL”的快路径，但缓存未命中、无效缓存和 Redis 读取异常都应回源 PostgreSQL。PostgreSQL 读取成功后，Redis 回填失败只记录日志，不阻断当前判定。这样在缓存删除或 Redis 临时异常时，中间件仍能依据事实源拒绝旧 token。

替代方案是 Redis 读取异常直接返回内部错误。该方案安全但可用性差，而且不能解决旧缓存写失败后的主动回源诉求，因此不采用。若 PostgreSQL 也不可用，系统仍应返回安全失败或既有错误映射。

### Decision: 保持现有分层和 Fx 注入

变更集中在 `user-services/internal/features/auth/app` 与 `user-services/internal/features/auth/infra/redis`。`ports.go` 可扩展 `AuthSessionStore` 和 `AuthSessionLifecycle` 的窄接口；`credential_store.go` 继续作为同一底层 PostgreSQL adapter 同时提供凭证和 token version 能力；`session_store.go` 继续只持有 Redis client、Redis key builder 和 TTL 配置。

替代方案是新增独立 outbox 表或 worker capability。该方案更适合强交付保障的异步补偿，但当前需求可通过幂等投影、日志和回源判定满足，不需要引入新的数据模型和部署组件。

## Risks / Trade-offs

- [Risk] Redis 投影失败后 refresh session 可能短时间仍存在。→ Mitigation: Access Token 通过 `token_version` 回源 PostgreSQL 拒绝；Refresh Token 校验也会比较当前 `token_version`，旧 refresh session 即使存在也无法刷新成功。
- [Risk] Redis 故障时中间件会增加 PostgreSQL 读取压力。→ Mitigation: 仅在缓存 miss/异常时回源；Redis 恢复后成功回填继续走快路径，可通过日志观察缓存故障频率。
- [Risk] 单调缓存写入脚本增加 Redis store 复杂度。→ Mitigation: 将脚本限制在 `infra/redis/session_store.go` 内，并补充 miniredis 或脚本级测试覆盖旧版本不覆盖新版本。
- [Risk] 改密返回成功但 Redis 投影失败可能让调用方误以为所有物理会话已立即删除。→ Mitigation: 规格明确安全状态以 PostgreSQL `token_version` 为准；旧 token 在后续校验中被拒绝，物理 Redis 删除通过幂等补偿完成。
- [Risk] 旧测试可能断言缓存回填失败会导致改密失败。→ Mitigation: 更新测试以覆盖新长期语义，并保留日志/补偿错误可观察性。

## Migration Plan

1. 扩展 app 层端口和 session lifecycle，新增按目标 `token_version` 执行 Redis 投影的方法。
2. 调整 `AuthService.ChangePassword`，使用凭证更新返回的 `token_version` 执行投影，不再二次递增。
3. 调整 `RevokeAllUserSessions`，保留退出全部设备的 PostgreSQL 递增，但 Redis 投影失败只记录和补偿，不回滚、不返回“状态未完成”。
4. 调整 `currentTokenVersion`，对 Redis miss/异常回源 PostgreSQL，回填失败不阻断当前判定。
5. 调整 Redis session store，增加单调 token version cache 写入和 cache 驱逐能力。
6. 更新 app 与 Redis infra 测试，覆盖单次递增、投影失败、cache 驱逐后回源和旧 token 拒绝。

回滚策略：如果新行为出现问题，可以恢复为旧的同步失败返回路径，但必须注意旧路径仍存在 PostgreSQL 已提交而 Redis 失败的不一致风险。由于不涉及 schema、migration 或外部契约变更，代码回滚即可。

## Open Questions

- 是否需要在后续 change 中引入持久化 outbox/后台 worker 来保证 Redis 投影最终完成并提供运维可观测面板？本次变更不引入。
- Redis cache 驱逐采用无条件删除还是条件删除旧版本即可满足需求；实现时优先选择更简单且安全的方案，并通过测试证明旧 cache 不会继续放行旧 token。
