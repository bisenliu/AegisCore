## Context

`user-services/internal/service/session_store.go` 负责 Redis 会话记录、用户活跃会话索引和 token version 缓存。当前实现将 `auth:user:<user_id>:sessions` 作为 Redis Set 使用，通过 `SAdd` 记录 `session_id`，并对整个索引 Key 设置 `Expire`。该方式只能让整个索引 Key 过期，不能按每个会话的真实过期时间清理成员；当用户存在多个不同过期时间的会话，或索引 Key TTL 被后续写入刷新时，已过期 session 的 member 可能继续留在索引中。

本变更属于 `user-session-control` capability，代码边界限定在 service 层的 session store，不改变 controller、repository、HTTP API、响应信封、数据库 schema 或 Ent 生成代码。

## Goals / Non-Goals

**Goals:**

- 将用户活跃会话索引升级为 Redis ZSet，Key 保持 `auth:user:<user_id>:sessions`，member 保持 `session_id`，score 使用会话过期时间 Unix 秒。
- 在所有涉及该索引的写入、读取和删除路径上主动执行 `ZRemRangeByScore`，按当前 Unix 秒清理已过期成员。
- 保持会话记录 `auth:session:<session_id>` 继续使用 Redis String + TTL，索引只负责定位该用户的候选 session。
- 将 `parseTokenVersion` 改为 `strconv.ParseInt(value, 10, 64)`，降低鉴权热路径整数解析成本。
- 保持现有 `SessionStore` 接口和业务语义不变。

**Non-Goals:**

- 不做旧 Redis Set 数据兼容、类型检测、`WRONGTYPE` 降级处理或迁移脚本。
- 不改变 Access Token、Refresh Token、token version 的 claims、TTL 配置或签发规则。
- 不改变 HTTP 路由、请求/响应结构、错误码或 `common/response.Envelope`。
- 不修改 PostgreSQL schema、Ent schema 或 Atlas migration。

## Decisions

1. 用户会话索引使用 ZSet，而不是继续 Set + Key TTL。

   ZSet 的 score 能表达每个 session 的独立过期时间，`ZRemRangeByScore key -inf now` 可在索引访问路径上按成员粒度清理冷数据。Set + Key TTL 只能清理整个索引 Key，后续登录刷新 Key TTL 后会让旧 member 继续残留，退出全部设备时需要扫描更多无效 `session_id`。

2. `CreateSession` 在同一个 Redis transaction pipeline 中写 session string、清理过期索引成员、写 ZSet member。

   推荐顺序为 `Set(sessionKey, data, ttl)`、`ZRemRangeByScore(userSessionsKey, -inf, now)`、`ZAdd(userSessionsKey, score=expiresAtUnix, member=sessionID)`。这样创建新会话时顺带清理该用户冷成员，且 session 记录和索引更新在同一次 `MULTI/EXEC` 内提交，减少中间状态窗口。索引 Key 不再需要依赖 `Expire`；是否额外设置 Key TTL 只可作为兜底释放空闲 Key，不应作为正确性来源。

3. 会话索引读取路径先清理，再读取有效 member。

   `DeleteAllUserSessions` 应先对用户索引执行 `ZRemRangeByScore` 清理过期 member，再通过 `ZRange` 或 `ZRangeByScore now +inf` 获取仍未过期的 session IDs，随后在 transaction pipeline 中删除这些 session string 并删除索引 Key。这样退出全部设备不会浪费大量 `DEL` 命令在已过期 session string 上。

4. 退出当前设备删除索引 member 时也执行过期清理。

   `DeleteSession` 应在同一个 transaction pipeline 中执行 `Del(sessionKey(sessionID))`、`ZRemRangeByScore(userSessionsKey(userID), -inf, now)`、`ZRem(userSessionsKey(userID), sessionID)`。这保证当前设备退出会移除目标 member，同时清理该用户所有已过期 member。

5. 过期清理使用当前 Unix 秒，score 使用 `session.ExpiresAt.Unix()`。

   `Session.ExpiresAt` 已是会话数据模型的一部分。创建会话时如果调用方传入的 `ExpiresAt` 与 `ttl` 不一致，实现应优先保证 score 表示实际会话过期时间；推荐在 `CreateSession` 内使用 `time.Now().Add(ttl).Unix()` 作为 ZSet score，并确保调用方构造 `Session.ExpiresAt` 与 TTL 一致，避免索引早删或晚删。

6. `parseTokenVersion` 使用 `strconv.ParseInt`。

   `fmt.Sscan` 是通用扫描器，会处理更多格式化场景并引入额外开销；token version 缓存值只需要十进制 int64，`strconv.ParseInt(value, 10, 64)` 更直接，错误语义也足以表达缓存值非法。

## Risks / Trade-offs

- [Risk] 部署环境若残留同名 Redis Set，ZSet 命令会返回 `WRONGTYPE`。→ Mitigation：该系统按全新上线处理，不实现兼容逻辑；上线前清空旧测试 Redis 或确保不存在同名 Set。
- [Risk] `Session.ExpiresAt` 与 Redis String TTL 不一致会导致索引清理时间与真实 session 生命周期偏离。→ Mitigation：实现中使用实际 `ttl` 计算 ZSet score，测试覆盖 score 与 TTL 的一致性；调用方继续保持 `ExpiresAt` 与 refresh token TTL 对齐。
- [Risk] `DeleteAllUserSessions` 读取索引和批量删除之间可能并发创建新会话。→ Mitigation：保持当前语义边界，退出全部设备的最终失效依赖 PostgreSQL `token_version` 递增；Redis 删除仅做会话缓存清理，新会话若携带旧 token version 也会被版本校验拒绝。
- [Risk] 每次索引访问增加一次 `ZRemRangeByScore`。→ Mitigation：该命令按 score 范围清理，能显著降低长期累积冷成员带来的后续扫描成本；写入和退出路径使用 pipeline 降低 RTT。

## Migration Plan

1. 修改 `session_store.go` 的 Redis 索引命令：`SAdd/SRem/SMembers` 替换为 `ZAdd/ZRem/ZRange` 或 `ZRangeByScore`，并在相关 pipeline 中加入 `ZRemRangeByScore`。
2. 修改 `parseTokenVersion` 和 imports：删除 `fmt.Sscan` 用法，增加 `strconv`。
3. 更新或新增 session store 测试，验证 ZSet member、score、过期清理、退出当前设备、退出全部设备和 token version 解析。
4. 运行 `go test ./...`，至少覆盖 `user-services` 模块；若 common 受影响则同时运行 common 测试。
5. 上线前确认 Redis 中不存在旧 Set 类型的 `auth:user:<user_id>:sessions` Key；本变更不包含兼容迁移。

## Open Questions

无。