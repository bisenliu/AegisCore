## Context

`user-session-control` 当前 Redis 会话实现位于 `user-services/internal/features/auth/infra/redis/session_store.go`，Redis key 由 `user-services/internal/features/auth/domain/rediskeys.go` 统一构造。现状 key 仍以旧格式表达：会话载荷按 `session_id` 独立存储，用户会话索引与 token version cache 使用 `auth:user:<user_id>:...` 结构，并且可选叠加 `config.App.Name` 前缀。

本变更要求直接切换到新格式：Session Key 为 `auth:session:{<userID>}:<sessionID>`，User Sessions Index 为 `auth:user:sessions:{<userID>}`，Token Version Key 为 `auth:user:token_version:{<userID>}`。原有 `config.App.Name` 命名空间前缀继续保留：当 app name 非空时，上述业务 key 前面仍拼接 `<app.name>:`；当 app name 为空时保持无前缀。这不是兼容性迁移，而是 Redis 数据契约的破坏性切换；实现不得保留旧格式读取、双写或迁移期分支。`{<userID>}` 同时作为 Redis Cluster hash tag，让单用户相关 key 能在 Lua 脚本中落到同一个 hash slot。Redis Cluster 的 hash slot 计算使用 key 中第一个有效 `{...}` 子串，服务名前缀位于 hash tag 外部，不影响同一用户多 key 脚本落入同一 slot。

当前 `RotateSession` 使用 WATCH/MULTI 乐观事务读取旧会话、写入新会话并删除旧会话。该方案虽然能表达原子提交，但在同一旧 Refresh Token 高并发刷新时容易产生 WATCH 冲突和客户端重试压力。`DeleteAllUserSessions` 当前在 Go 侧读取索引成员后通过 pipeline 批量 `DEL`，大量 session payload 删除可能阻塞 Redis 主线程。

## Goals / Non-Goals

**Goals:**

- 将认证 Redis key 构造统一切换到新格式，并保留原有 `config.App.Name` 前缀语义。
- 使用 Redis Lua 脚本实现 `RotateSession`，在 Redis 内原子完成旧会话校验、新会话写入、索引更新、旧会话删除和过期索引清理。
- 使用 Redis Lua 脚本实现 `DeleteAllUserSessions`，并通过 `UNLINK` 异步释放会话 payload key 和用户索引 key。
- 保持 `transport/http`、`app` service、JWT claims、HTTP API、响应信封、PostgreSQL schema 和 Atlas migration 不变。
- 更新 Redis session store 和 key builder 测试，明确旧 key 不再被读写。

**Non-Goals:**

- 不提供旧 Redis key 的迁移脚本、兼容读取、回填或双写。
- 不新增用户会话列表、设备管理、认证管理 API 或 Redis 统计能力。
- 不修改 Ent schema、生成代码、Atlas migration 或 PostgreSQL token version 字段语义。
- 不把 Lua 脚本细节泄漏到 controller 或 auth service；脚本只属于 Redis infra adapter。

## Decisions

### Decision 1: Redis key builder 保留服务名前缀并生成新格式业务 key

`RedisKeyBuilder` 保留在 auth domain 包中作为 key 命名的唯一入口，但方法签名需要按新格式调整：`AuthSession` 应接收 `userID` 和 `sessionID`，返回 `<prefix>:auth:session:{<userID>}:<sessionID>`；`AuthUserSessions` 返回 `<prefix>:auth:user:sessions:{<userID>}`；`AuthUserTokenVersion` 返回 `<prefix>:auth:user:token_version:{<userID>}`。`<prefix>` 继续由 `config.App.Name` 去除首尾空白后决定，非空则拼接到业务 key 前，空值则省略。

选择该方案是因为用户明确要求保留原有按服务名拼接的逻辑，只替换服务名之后的业务 key。服务名前缀不影响 Redis Cluster 脚本要求：所有参与同一 Lua 脚本的 key 都包含相同 `{<userID>}` hash tag，Redis Cluster 会使用 hash tag 内部的 user ID 计算 slot，而不是使用完整 key 前缀。替代方案是去掉 app name 前缀；这会改变既有命名空间隔离语义，且对 Cluster 同 slot 没有额外收益。

### Decision 2: `RotateSession` 使用单个 Lua 脚本提交

Redis adapter 在 Go 侧继续负责 TTL 兜底、过期时间推导和 session payload JSON 序列化；Lua 脚本负责持久化边界内的原子动作。脚本输入应包含旧 session key、新 session key、用户会话索引 key、旧 session id、新 session id、新 session JSON、会话 TTL 秒数或毫秒数、新会话过期 Unix 时间、当前 Unix 时间和索引 TTL。

脚本执行顺序为：读取旧 session payload；不存在则返回 `not_found`；校验 payload 中旧 session 的 `user_id`、`session_id` 和 `token_version` 与调用方预期一致，不一致返回 `mismatch`；写入新 session key 并设置 TTL；清理用户索引中过期 member；写入新 session member；移除旧 session member；删除旧 session key；仅当索引当前 TTL 小于目标索引 TTL 时延长索引 TTL。Go 层将返回码映射为 `authdomain.ErrAuthSessionNotFound`、`authdomain.ErrAuthSessionMismatch` 或包装后的 Redis 错误。

选择 Lua 而不是 WATCH，是为了把高并发冲突处理留在 Redis 单线程脚本执行队列里，避免客户端围绕 WATCH 冲突形成重试风暴。选择不在 service 层加锁，是因为进程内锁无法覆盖多实例部署。

### Decision 3: `DeleteAllUserSessions` 使用 Lua + `UNLINK`

`DeleteAllUserSessions` 的脚本以用户索引 key 和 session key 前缀为输入，在 Redis 内先按当前 Unix 时间清理过期索引 member，再读取仍未过期的 session id，拼出 `<prefix>:auth:session:{<userID>}:<sessionID>` key 列表，并调用 `UNLINK` 异步删除这些 payload key 以及索引 key。脚本返回被调度删除的 key 数量仅用于测试或调试，业务层仍以 error 为准。

选择 `UNLINK` 而不是 `DEL`，是为了避免大量会话 payload 释放内存时阻塞 Redis 主线程。替代方案是在 Go 侧分批 `UNLINK`，但那会重新引入读取索引和删除 key 之间的命令间窗口，也无法保证索引清理和删除调度的原子性。

### Decision 4: 分层边界保持不变

`AuthService` 和 `authSessionLifecycle` 继续只通过 `AuthSessionStore` 表达 `CreateSession`、`RotateSession`、`GetSession`、`DeleteSession` 和 `DeleteAllUserSessions`。Lua 脚本常量、脚本返回码和 Redis 命令封装只放在 `infra/redis` 包内。`transport/http` 不接触 Redis key、Lua 或 token version 细节，`app` 层不导入 Redis client 或 Ent 生成包。

选择该方案是为了符合仓库现有 capability 分层规则：业务编排在 app，Redis 访问在 infra/redis，key 命名契约由 auth domain 稳定承载。

### Decision 5: 测试以 key 契约和原子行为为中心

需要更新 `rediskeys_test.go`，断言服务名前缀和新 key 精确格式可以同时生效，并断言 app name 为空时保持无前缀。`session_store_test.go` 应更新所有直接 key 断言，覆盖 `CreateSession`、`GetSession`、`DeleteSession`、`RotateSession`、并发轮换只成功一次、`DeleteAllUserSessions` 删除全部未过期 session 和索引。额外增加旧 key 不被读取的断言：预置旧格式 Redis key 后，新实现应报告 cache miss 或 session not found。

集成层不需要新增 HTTP API 测试，因为路由、DTO 和响应契约不变；本变更风险集中在 Redis adapter。

## Risks / Trade-offs

- [Risk] 直接切换 key 会使旧 Redis session 和旧 token version cache 失效，用户可能需要重新登录。→ Mitigation：在 proposal、spec 和部署说明中明确这是破坏性切换，不提供兼容路径。
- [Risk] Lua 脚本解析 JSON 字符串比 Go 代码脆弱。→ Mitigation：脚本只校验简单字符串字段，Go 层仍负责 JSON marshal/unmarshal；脚本返回码必须有单元测试覆盖。
- [Risk] Redis Cluster 要求 Lua 脚本 KEYS 位于同一 hash slot。→ Mitigation：单用户相关 key 均包含 `{<userID>}` hash tag，Rotate 和 DeleteAll 仅操作同一用户的 key。
- [Risk] `UNLINK` 是异步释放，测试中 key 逻辑删除和内存释放时间不是同一件事。→ Mitigation：业务只依赖 key 不再存在；测试断言 key 不存在，不断言内存释放完成。
- [Risk] miniredis 对 Lua 或 UNLINK 支持可能与真实 Redis 有差异。→ Mitigation：优先使用 go-redis `Script`/`Eval` 的常规命令组合，并在必要时用可被 miniredis 支持的脚本特性；不因测试替身限制改变生产语义。
