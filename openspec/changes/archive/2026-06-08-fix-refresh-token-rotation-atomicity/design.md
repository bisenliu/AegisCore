## Context

`user-session-control` 的刷新流程位于 `user-services/internal/service/auth_service.go`。当前 `Refresh` 在解析 Refresh Token、校验 Redis session 和 token version 后，如果启用了 `refreshTokenRotation`，会先调用 `s.sessions.DeleteSession(ctx, claims.UserID, session.SessionID)` 删除旧会话，再生成新的 `sessionID` 并调用 `issueTokenPair`。`issueTokenPair` 会先签发 Access Token/Refresh Token，再调用 `s.sessions.CreateTokenSession` 写入新 Redis session。

这形成了明显的非原子性窗口：旧 session 删除已经持久生效，但新 token 或新 session 尚未确认可用。如果 token 签发失败，响应中不会返回新 Refresh Token；如果 Redis 新 session 写入失败，即使新 token 已在内存中生成也不会成功返回，且该新 Refresh Token 对应服务端 session 不存在。两种情况下，用户持有的旧 Refresh Token 已被撤销，新 Refresh Token 又不可用，后续刷新会被视为会话不存在，用户被动掉线。

在启用 RefreshTokenRotation 的前提下，这属于高风险实现缺陷：触发条件并不依赖恶意输入，只需要刷新请求通过校验后发生 JWT 签发错误、Redis 写入错误、网络抖动、超时或 Redis 局部故障即可触发。该问题同时损害用户体验和系统一致性，也会让内部依赖短暂故障放大为认证会话丢失。

## Goals / Non-Goals

**Goals:**

- 保证 RefreshTokenRotation 启用时，成功响应中的新 Refresh Token 必须对应可用的新 Redis session。
- 避免在新 token 和新 session 不可用前提前撤销旧 session，降低内部失败导致用户被动掉线的风险。
- 明确旧 session 删除失败、新 session 创建失败和 token 签发失败的错误处理语义。
- 为后续严格防重放方案保留 Redis 事务或 Lua 脚本的实现路径。

**Non-Goals:**

- 不改变认证 HTTP API 路由、请求体、响应信封或错误码契约。
- 不改变 JWT claims、Redis key 格式、Refresh Token TTL、token version 语义或数据库 schema。
- 不引入新的认证能力、权限体系或用户管理功能。
- 不手写 Ent 生成代码，不生成 Atlas migration。

## Decisions

1. 将轮换默认实现调整为“先确保新凭据可用，再撤销旧 session”。

   刷新流程在旧 session 校验通过后生成新 `sessionID`，先签发新 token pair 并创建新 Redis session；只有这两步均成功后，才删除旧 session。这样 token 签发失败或新 session 写入失败时，旧 Refresh Token 仍然保留，调用方可以重试刷新，不会因内部失败被动掉线。

   备选方案是继续先删旧 session 并在失败时尝试恢复旧 session。该方案需要可靠恢复旧 session 的 TTL、payload 和用户活跃会话索引状态，且恢复失败仍会掉线，因此不作为首选。

2. 将旧 session 删除失败视为刷新失败，但不得返回新 token。

   如果新 session 已创建但旧 session 删除失败，直接返回新 token 会导致旧 Refresh Token 和新 Refresh Token 同时可用，削弱轮换的防重放目标。因此此时应返回失败，并尽量清理刚创建的新 session 作为补偿，避免残留会话扩大可用凭据数量。补偿清理失败应记录日志，但不应泄漏 token 给调用方。

   备选方案是允许短时间双 session 共存，以换取可用性。但在启用 RefreshTokenRotation 的语义下，旧 token 应被撤销，双有效窗口会弱化安全收益，因此只适合作为明确接受的业务折中，不作为默认语义。

3. 如安全目标要求严格防重放，应新增 repository 层原子轮换能力。

   更严格的做法是在 Redis repository 中使用事务或 Lua 脚本原子执行：确认旧 session 仍存在且匹配、创建新 session、写入用户活跃会话索引、删除旧 session、移除旧索引 member、清理过期索引 member。这样可避免并发刷新或中间失败导致的双 session 或零 session 状态。

   备选方案是在 service 层串行调用 Redis 命令。该方案改动小，但无法消除 Redis 命令之间的并发和失败窗口，只能降低“新凭据不可用时旧凭据已失效”的可用性风险，不能提供严格原子轮换保证。

4. 保持分层边界。

   `AuthService` 继续负责认证用例编排和错误语义选择；token 签发仍由认证 token 组件负责；Redis session 创建、删除或原子轮换能力由认证会话组件和 repository 抽象承载。controller 不参与 JWT claims、Redis session 或 token version 的业务校验。

## Risks / Trade-offs

- [Risk] 先创建新 session 再删除旧 session 会在旧 session 删除失败时产生短暂双 session 风险。→ Mitigation：删除旧 session 失败时不返回新 token，并补偿删除新 session；对需要严格防重放的场景使用 Redis Lua/事务实现原子轮换。
- [Risk] 补偿删除新 session 也可能失败，Redis 中可能残留调用方未收到的 session。→ Mitigation：记录带 trace-id 的告警日志，并依赖 Refresh Token TTL 与用户会话索引清理策略最终清理；如风险不可接受，升级为原子轮换实现。
- [Risk] Lua/事务方案会增加 Redis repository 复杂度。→ Mitigation：将原子轮换封装在 repository 抽象中，service 层只表达业务动作，测试覆盖脚本成功、旧 session 不存在、并发轮换和失败路径。
- [Risk] 并发使用同一个旧 Refresh Token 发起刷新时，非原子 service 编排仍可能出现竞态。→ Mitigation：把非原子重排视为最低修复，若 RefreshTokenRotation 的安全目标包含强防重放，则实现 Redis 原子比较并交换式轮换。
