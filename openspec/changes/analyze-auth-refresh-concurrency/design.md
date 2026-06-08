## Context

当前 `AuthService.Refresh` 位于 `user-services/internal/service/auth_service.go`，流程为：规范化请求、解析 Refresh Token、通过 `AuthSessionLifecycle.ValidateRefreshSession` 校验 Redis 会话和 `token_version`、根据 `auth.refresh_token_rotation` 决定复用旧 `session_id` 或创建新 `session_id`、签发 token pair、创建新 Redis 会话、删除旧 Redis 会话，并在删除旧会话失败时尝试删除新会话。

该实现已经满足“新 token 签发失败或新会话创建失败时不提前删除旧会话”的基本顺序要求，但轮转流程仍由 service 层手动串联多个步骤：`GetSession`、`CreateSession`、`DeleteSession` 分别执行，缺少一个能表达“消费旧 refresh 会话并创建新 refresh 会话”的单一领域动作。代码上，`Refresh` 方法包含轮转和非轮转两条路径、失败补偿和多层条件分支，可读性随安全场景增加而下降。

并发方面，当前 Redis 仓储的 `CreateSession` 与 `DeleteSession` 各自使用 pipeline，但 Refresh 轮转整体不是原子的。两个请求如果几乎同时使用同一个旧 Refresh Token：两者都可能在旧 session 删除前通过 `ValidateRefreshSession`，随后各自创建新 session，最终都删除同一个旧 session 并返回不同的新 Refresh Token。这会形成重复刷新和多新会话可用的窗口，不能满足严格重放防护。进程内 `sync.Mutex` 只能保护单实例，无法覆盖多副本部署，因此不应作为主要并发控制方案。

## Goals / Non-Goals

**Goals:**

- 让 `AuthService.Refresh` 只保留高层用例编排，避免把 token 解析、会话校验、轮转写入和失败补偿全部堆叠在一个方法中。
- 将 Refresh Token 轮转抽象为 session lifecycle 或 repository 边界上的明确动作，减少 service 层手动双向删除和多层 `if` 嵌套。
- 在启用轮转时提供跨服务实例有效的原子性保障，避免同一个旧 Refresh Token 并发刷新成功多次。
- 保持现有 HTTP API、错误映射、JWT claims、Redis key 格式、配置项和数据库模型不变。

**Non-Goals:**

- 不新增认证接口、管理接口或用户会话列表能力。
- 不修改 Ent schema、Atlas migration、用户表结构或 Redis key 命名规则。
- 不把 Redis Lua 脚本、事务细节泄漏到 controller 或 service 编排层。
- 不依赖进程内互斥锁作为分布式并发安全边界。

## Decisions

### Decision 1: 拆分 Refresh 内部编排

推荐将 `Refresh` 拆成以下私有方法或等价结构：

- `validateRefreshRequest(ctx, req)` 或保留请求规范化在 `Refresh` 顶层，保证入口先失败返回。
- `parseAndValidateRefreshSession(ctx, refreshToken)`：组合 `tokens.ParseRefreshToken` 与 `sessions.ValidateRefreshSession`，返回 claims、旧 session 和当前 token version。
- `refreshWithoutRotation(ctx, claims, session, currentVersion)`：复用旧 `session_id` 签发 token pair。
- `refreshWithRotation(ctx, claims, session, currentVersion)`：只负责轮转路径的高层编排，内部不展开 Redis 多命令细节。
- `issueTokenPair(ctx, userID, tokenVersion, sessionID)` 继续作为签发和创建 session 的共享 helper；如果原子轮转方案要求先签 token 再原子写 session，则可新增专用 helper 避免混用旧语义。

理由：该拆分能让 `Refresh` 变成“校验输入 -> 校验会话 -> 根据配置选择策略”的线性流程，并使轮转失败补偿逻辑集中在单一方法中。替代方案是在当前方法中继续加注释或提前返回；这能局部改善阅读体验，但无法阻止后续并发保护逻辑继续堆叠在同一方法里。

### Decision 2: 原子轮转放在 repository 抽象内

推荐在 `repository.AuthSessionRepository` 新增原子轮转能力，例如 `RotateSession(ctx, oldSession, newSession, ttl)` 或更明确的 `RotateSession(ctx, userID, oldSessionID, newSession AuthSession, ttl)`。Redis 实现使用 Lua 脚本或 `WATCH`/事务完成以下不可分割步骤：确认旧 session key 仍存在且归属用户和 token version 匹配、写入新 session key、更新用户会话 ZSet、删除旧 session key、从 ZSet 移除旧 session，并清理过期 ZSet member。

理由：Redis 是会话状态事实源，原子操作必须靠 Redis 自身的单线程脚本或乐观事务保障，才能覆盖多 goroutine、多进程和多服务实例。替代方案包括：service 层 `sync.Mutex`、按用户本地锁、先删旧再建新、继续保留现有创建后删除顺序。前两者无法覆盖分布式部署；先删旧再建新会在签发或写入失败时破坏旧 token 可重试语义；继续当前顺序会允许并发重复刷新。

### Decision 3: token 签发与原子写入的失败语义保持兼容

JWT 签发本身无法和 Redis 写入组成同一个事务。推荐顺序为：先解析并校验旧 Refresh Token，生成新 `session_id`，签发新 token pair；签发成功后调用 repository 原子轮转。如果原子轮转失败，service MUST 不返回新 token，且记录安全相关失败日志。因为新 token 对应的新 session 未被原子提交或旧 session 已被其他请求消费，所以新 Refresh Token 即使已经在内存中生成，也不会暴露给调用方。

理由：该顺序保留“新 token 签发失败时旧 session 不被撤销”的既有语义，并通过原子提交避免“新旧会话同时有效”。替代方案是先预留 Redis session 再签 token，但签发失败时仍需补偿删除新 session，复杂度更高，且不能解决旧 session 并发消费问题。

### Decision 4: 测试覆盖真实风险场景

需要新增测试验证同一旧 Refresh Token 的并发刷新或顺序模拟竞争只会有一个成功。repository 层测试应覆盖原子轮转成功、旧 session 缺失、旧 session user/token_version 不匹配、并发调用只提交一次、ZSet 索引更新一致。service 层测试应覆盖轮转成功、签发失败保留旧 session、原子轮转失败不返回新 token、轮转关闭时继续复用旧 `session_id`。

理由：当前单元测试覆盖了部分失败补偿，但没有证明跨 `ValidateRefreshSession`、`CreateSession`、`DeleteSession` 的竞争安全。替代方案只靠代码审查判断并发安全，无法防止后续重构回归。

## Risks / Trade-offs

- 原子轮转方法需要扩展 repository 接口，测试 stub 和 Fx 依赖会同步调整。缓解：先在 service lifecycle 内封装新方法，再小范围更新现有测试替身。
- Redis Lua 脚本可读性低于普通 Go pipeline。缓解：将脚本限定在 Redis repository 实现中，并为每个返回码建立清晰的 Go 错误映射和测试。
- `WATCH`/事务在高并发下可能产生重试或失败。缓解：优先使用 Lua 脚本；如选择 WATCH，限制重试次数并把冲突映射为 token invalid 或 retryable internal error。
- 新 token 已签发但原子轮转失败时会被丢弃。缓解：不向调用方返回，且没有可用 Redis session 支撑该 Refresh Token，外部行为保持失败。
- 只补强 Refresh 轮转不会解决所有用户级安全事件的跨 Redis/PostgreSQL 原子性。缓解：本变更仅处理 Refresh Token 轮转；`LogoutAll` 和改密的 PostgreSQL 与 Redis 一致性保持既有语义。
