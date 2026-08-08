## Context

当前 auth refresh session 全量撤销采用两阶段删除：在线路径 detach 用户 session 索引，随后通过 auth 自有 `workerpool` 异步清理 detached session key。`DeleteAllUserSessions` 在提交后台任务时将 `purge_key` 与 `session_prefix` 放入 `Task.Fields`，而 `common/runtime/workerpool` 在任务返回 error 或 panic 时会把 `Task.Fields` 原样写入日志。

`purge_key` 和 `session_prefix` 都由 auth Redis key catalog 构造，包含 Redis namespace、`auth` scope、用户 UUID hash tag，并且 `session_prefix` 可与 session ID 拼接为完整 refresh session key。该行为违反 workerpool 对 `Task.Fields` 不得包含完整 Redis key 或敏感值的共享契约。

本 change 只收敛 auth purge workerpool 的日志字段和测试，不改变 Redis key schema、session 撤销语义、workerpool API、HTTP API、OpenAPI、数据库 schema、migration 或部署资产。

## Goals / Non-Goals

**Goals:**

- auth purge workerpool 的 error 和 panic 日志 MUST NOT 包含完整 Redis key、Redis key prefix、Redis namespace、用户 UUID hash tag 或可拼装 refresh session key 的材料。
- 后台任务执行仍 MUST 使用既有 `purgeKey` 与 `sessionPrefix` 完成 Redis 清理，但这些值只允许作为闭包内部数据，不允许进入 `Task.Fields`。
- 日志定位 MUST 依赖稳定任务名、低敏批量大小、cut time，以及确需关联时的不可逆 opaque 标识。
- 测试 MUST 覆盖后台任务失败和 panic 两条 workerpool 日志路径，验证敏感 key material 不进入日志。
- 不保留旧日志字段，不提供兼容字段名或兼容分支。

**Non-Goals:**

- 不修改 Redis key catalog、Redis key 实际格式或存量 Redis 数据。
- 不修改 token version、refresh session rotation、退出全部会话或强制改密的安全语义。
- 不修改 `common/runtime/workerpool` 公开 API 或日志写入机制。
- 不新增通用日志脱敏框架、全局 zap core filter 或服务级兼容配置。
- 不修改 HTTP API、OpenAPI、Ent schema、Atlas migration、deployment manifests 或 observability dashboard。

## Decisions

1. 在 auth 消费端移除敏感字段，而不是在 workerpool 中做通用过滤。

   原因：workerpool 是业务中立 primitive，无法可靠判断调用方字段中的 Redis key、token、业务 ID 或其他敏感值；其文档已经将字段安全责任交给消费端。把修复放在 `user-service/internal/features/auth/infrastructure/redis` 能保持 common API 稳定，并避免引入无法覆盖所有业务字段的启发式过滤。

   备选方案：在 workerpool 内按字段名过滤 `purge_key`、`session_prefix`。该方案会形成字段名黑名单，无法覆盖其他 key 字段，也会隐藏消费端违反契约的问题，因此不采用。

2. 后台清理闭包继续捕获完整 Redis key material，但 `Task.Fields` 只保留低敏观测字段。

   原因：Redis 清理本身必须依赖 `purgeKey` 和 `sessionPrefix`，改变这些参数会影响删除语义；风险来自日志输出而不是运行时使用。最小正确修复是缩小 `Task.Fields`，保留 `Name`、`batch_size`、`cut_time` 等低敏定位信息。

   备选方案：重构 purge 函数，让任务内部重新计算 key。该方案不能降低日志风险，且会增加参数与时间点不一致的复杂度，因此不采用。

3. 明文 `user_id` 不进入该后台任务的 workerpool 错误字段。

   原因：当前泄露链中的高风险内容是 Redis hash tag 中的用户 UUID。为了保证验收标准在 error 和 panic 日志中都不出现 `{user_uuid}` 或可拼装 key material，并降低集中日志中的用户标识扩散，该任务的 workerpool 字段不再携带明文用户 ID。需要关联时使用不可逆 opaque 标识。

   备选方案：保留 `user_id`，只删除 Redis key 字段。该方案可以消除完整 key 泄露，但仍会让后台故障日志大规模复制用户标识；本 change 的目标是更严格的日志最小化，因此不采用。

4. 如需关联单次 purge，使用不可逆 opaque 标识，不使用可逆编码、截断 key 或 session prefix。

   原因：opaque 标识能在不暴露可操作 key material 的前提下辅助同一批次日志关联。实现时可基于 purge key 或 purge ID 计算固定长度 HMAC/SHA-256 摘要；若没有服务内稳定 secret，普通 hash 只能用于低风险关联，不应被描述为强安全脱敏。

   备选方案：记录截断后的 purge key 或只隐藏 session ID。该方案仍泄露 namespace、scope 和用户 hash tag，不能满足验收标准，因此不采用。

5. 使用日志捕获测试验证失败和 panic 路径。

   原因：风险触发点是 workerpool 在 error/panic 时拼接 `Task.Fields` 后输出日志，单纯测试字段构造不足以证明最终日志安全。测试应通过 zap observer 或等价局部 logger 捕获日志，并断言字段名和值均不包含敏感 key material。

   备选方案：只增加静态字符串测试。该方案不能覆盖 workerpool 日志拼接路径，因此不采用。

## Risks / Trade-offs

- [Risk] 移除 `purge_key`、`session_prefix` 和明文 `user_id` 后，单次 purge 故障的人工定位信息减少。→ Mitigation：保留稳定任务名、`batch_size`、`cut_time`，并在确需关联时使用不可逆 opaque 标识。
- [Risk] 普通 SHA-256 摘要如果输入空间可枚举，不能提供强脱敏保证。→ Mitigation：实现时优先使用服务内 HMAC secret；若不具备 secret，则仅将摘要定位为低敏关联 ID，不作为敏感数据脱敏边界。
- [Risk] 测试如果依赖固定 `time.Sleep` 等待后台任务日志，可能不稳定。→ Mitigation：使用 `require.Eventually`、通道或 workerpool 可观察状态等待日志出现。
- [Risk] 只修复 auth 当前调用点，未来其他 workerpool 消费端仍可能传入敏感字段。→ Mitigation：本 change 同步补充 shared-platform-primitives 规格，要求消费端任务字段遵守 workerpool 日志安全契约；后续新增调用点按该规格审查。

## Migration Plan

1. 修改 auth purge task 的 `Task.Fields`，删除 `purge_key`、`session_prefix` 和明文 `user_id`，保留稳定低敏字段，并按需加入不可逆 opaque 标识。
2. 增加或更新 auth Redis session purge 测试，分别注入后台 purge error 和 panic，捕获 workerpool 日志并断言不包含 Redis namespace、`auth:session`、`auth:user:sessions`、`{user_uuid}`、`purge_key`、`session_prefix` 或可拼装 session key material。
3. 运行相关包测试，优先覆盖 `user-service/internal/features/auth/infrastructure/redis` 和 `common/runtime/workerpool` 相关测试。
4. 运行 `make user-service-architecture-lint` 验证 OpenSpec 与架构文档约束。

回滚方式：如果实现导致测试或排障能力不可接受，回滚本 change 的代码和规格变更；不得通过恢复 `purge_key`、`session_prefix` 或明文 Redis key material 的日志字段作为回滚方案。

## Open Questions

无。
