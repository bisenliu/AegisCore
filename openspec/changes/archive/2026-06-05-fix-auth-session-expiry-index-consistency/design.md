## Context

`user-services/internal/repository/redis/auth_session_repository.go` 负责通过 Redis 保存 Refresh Token 会话记录和用户活跃会话索引。当前 `CreateSession` 会在同一个 `TxPipeline` 中执行 `SET auth:session:<session_id> ... ttl`、`ZREMRANGEBYSCORE auth:user:<user_id>:sessions -inf now` 和 `ZADD auth:user:<user_id>:sessions session.ExpiresAt.Unix() session_id`。

问题成因是会话生命周期存在两个来源：session key 的实际过期由传入 `ttl` 决定，ZSet score 由 `session.ExpiresAt` 决定。若调用方传入的 `ExpiresAt` 与 `ttl` 不一致，Redis key 可能已过期但 ZSet member 仍显示未过期，或 ZSet member 先被清理但 session key 仍存在。与此同时，用户会话 ZSet 本身没有 TTL，只依赖写入、删除和批量删除路径中的懒清理；一旦 score 与真实 session key TTL 脱节，已过期 `session_id` 可能长期残留。

影响集中在 `user-session-control` capability 的 repository 层 Redis 实现，不涉及 controller/service HTTP 契约、PostgreSQL schema、Ent 生成代码或 common 共享基础设施。

## Goals / Non-Goals

**Goals:**

- 让 Redis session key 的 TTL、序列化会话中的 `ExpiresAt`、用户会话 ZSet score 使用同一个过期时间来源。
- 降低 ZSet 中过期或无对应 session key 的 `session_id` 长期残留风险。
- 避免 `DeleteAllUserSessions` 遍历大量过期残留，控制 Redis 存储和操作成本增长。
- 为未来会话统计、管理和审计能力提供更干净的活跃会话索引语义。
- 保持现有认证 API、Redis key 命名、响应信封和 token version 语义兼容。

**Non-Goals:**

- 不新增会话管理 API、审计报表或后台定时清理任务。
- 不修改 PostgreSQL 数据模型、Ent schema 或 Atlas migration。
- 不改变 Refresh Token 签发、校验、轮转和 token version 的外部行为。
- 不把 Redis session repository 迁移到 common 模块；该逻辑仍属于用户服务会话控制实现边界。

## Decisions

1. 以实际 Redis TTL 推导会话过期时间，消除双重来源。

   `CreateSession` 应先规范化 `ttl`，使用单次 `now := time.Now()` 计算 `expiresAt := now.Add(ttl)`，并将 `session.ExpiresAt` 覆盖为该值后再序列化。session key 的 `SET` TTL、ZSet score 和会话 payload 中的 `ExpiresAt` 都来自该 `expiresAt`。

   该方案优先于信任调用方传入的 `session.ExpiresAt`，因为 Redis key 的真实生命周期最终由 `SET` TTL 决定；让派生字段跟随 TTL 更符合当前接口已显式传入 `ttl` 的实现边界。若未来希望由绝对过期时间驱动，可再调整 repository 抽象为只接收 `ExpiresAt` 并从中计算 TTL。

2. 保留懒清理，并为用户会话 ZSet 设置随最长活跃会话延长的 key TTL。

   每次创建会话时继续执行 `ZREMRANGEBYSCORE` 清理已过期 member，并在写入新 member 后对 `userSessions` ZSet 设置过期时间。ZSet key TTL 应至少覆盖新会话 TTL，并可增加一个小的缓冲窗口，避免最后一个活跃会话刚过期时索引 key 长期存在。后续创建更长生命周期会话时允许延长 ZSet key TTL。

   该方案不要求后台任务，部署复杂度低，适合当前只有登录、刷新、退出当前设备和退出全部设备等入口的实现。它不能立即清理所有历史脏数据，但会在用户后续会话操作和 key 过期时逐步收敛。

3. 批量删除前清理过期 member，并只对清理后的索引执行删除。

   `DeleteAllUserSessions` 当前已先执行 `ZREMRANGEBYSCORE` 再 `ZRANGE`，应继续保持该顺序。修复后由于 ZSet score 与 session key TTL 统一，按 score 清理后的残留显著减少，批量删除不再稳定遍历已过期 sessionID。若历史数据中存在 score 未过期但 session key 已消失的脏 member，`DEL` 不存在 key 是幂等的，最终 `DEL userSessions` 会清空该用户索引。

4. 默认使用 `TxPipeline`，将 Lua 脚本作为更强一致性的可选升级。

   当前三类写操作在单个 Redis 实例上使用 `TxPipeline` 发送 `MULTI/EXEC`，可以保证命令批量执行顺序和事务内原子执行，但不能封装复杂条件校验，也无法在命令入队前防止调用方传入不一致字段。对于本次修复，先在 Go 层统一过期时间来源，并在事务中增加 ZSet key TTL 设置即可解决主要不一致。

   如果后续需要更强一致性，例如要求 `SET`、过期清理、`ZADD`、`EXPIRE` 作为不可拆分的 Redis 端语义，或者需要按 Redis server time 统一时间，应引入 Lua 脚本。Lua 的代价是测试和维护复杂度上升，脚本参数、时间来源和错误处理需要更严格封装；在当前风险规模下不作为首选实现。

## Risks / Trade-offs

- 调用方传入的非零 `session.ExpiresAt` 将被 repository 规范化覆盖 -> 通过测试明确 `ttl` 是会话生命周期的单一来源，并保持 service 层传入 TTL 的现有调用方式。
- ZSet key TTL 可能在仍有更长会话时被较短会话创建覆盖 -> 设置 TTL 时应使用不缩短现有 TTL 的策略，或在无法可靠使用 `EXPIRE GT` 时选择大于等于 refresh token 最大 TTL 的索引 TTL 缓冲策略。
- 历史 ZSet 中可能已有 score 与真实 key TTL 不一致的脏数据 -> 继续在创建、退出当前设备和退出全部设备路径懒清理；`DeleteAllUserSessions` 最终会删除该用户索引，避免继续保留该用户历史残留。
- Lua 脚本可提升 Redis 端一致性但增加复杂度 -> 本次实现优先最小修改；仅当测试或后续需求证明 `TxPipeline` 不足时再升级。
- 用户长时间无会话操作时，旧无 TTL ZSet 可能不会立即消失 -> 新会话写入会为索引设置 TTL；对完全沉默的历史用户，可通过后续运维脚本或后台清理能力单独处理，不纳入本变更目标。
