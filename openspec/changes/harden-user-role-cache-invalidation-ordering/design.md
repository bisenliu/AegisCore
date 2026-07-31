## Context

RBAC 授权热路径通过 `user-service/internal/features/permission/infrastructure/casbin` 中的用户角色 resolver 查询用户当前启用角色。cache 启用时，`RolesForUser` 通过 `common/runtime/localcache.LoadingCache` 合并同一用户的并发回源，并在命中或回源后返回复制后的 `[]uuid.UUID`；cache 关闭时直接回源并保持 fail-closed。

当前 `InvalidateUserRole` 和 `InvalidateAllUserRoles` 只删除本地缓存项或清空缓存。如果某次 cache miss 已经开始回源，随后用户角色 Add、Remove 或 Replace 成功并触发失效，旧回源仍可能在失效之后完成并写回旧角色集合。该风险位于 permission infrastructure 内，不需要改变 HTTP API、数据库 schema、OpenAPI、部署资产或 Casbin policy reload revision 门禁。

## Goals / Non-Goals

**Goals:**

- 在 permission user-role resolver 或其 cache wrapper 中引入 generation/revision 门禁，保证失效后完成的旧回源结果不能写入用户角色缓存。
- 使 `InvalidateUserRole` 先提升指定用户 generation 再删除对应缓存项。
- 使 `InvalidateAllUserRoles` 先提升全量 generation 再清空缓存，并抑制所有全量失效前启动的旧 load 写回。
- 保持 `RolesForUser` 返回独立 slice、cache disabled 直接回源、回源错误 fail-closed 和现有缓存统计语义。
- 通过并发测试和 race 测试覆盖 cache miss 与用户角色写并发、旧 load 回填抑制、全量失效和 cache disabled 模式。

**Non-Goals:**

- 不改变 Casbin policy 全量 reload 的 revision 门禁、Redis policy version、Pub/Sub watcher 或周期性补偿语义。
- 不实现 outbox dispatcher、watcher revision lag 或跨副本强一致角色缓存协议。
- 不把 RBAC 业务 generation/revision 语义加入 `common/runtime/localcache` 公共 API。
- 不保留允许旧 load 在失效后写回缓存的兼容分支。

## Decisions

1. 在 permission infrastructure 内维护 user-role generation 门禁。

   `entUserRoleResolver` 或同包私有 wrapper 维护一个全量 generation 和按用户 generation。cache load 开始时捕获 `(globalGeneration, userGeneration)`，回源完成后、交给 loading cache 写入前再次读取当前 generation；如果任一 generation 已变化，load MUST 返回可诊断错误并不得写入旧值。该方案把 RBAC 业务顺序语义留在 `user-service/internal/features/permission/infrastructure/casbin`，不污染 `common/runtime/localcache`。

   备选方案是扩展 `common/runtime/localcache`，提供带 revision 的条件写入 API。该方案会把 user-role 业务 revision 约束扩散到跨服务 primitive，当前没有独立 shared primitive 需求，因此不采用。

2. 失效操作先提升 generation，再执行 cache 删除或清空。

   `InvalidateUserRole(userID)` 先提升该用户 generation，再调用 cache delete；`InvalidateAllUserRoles()` 先提升全量 generation，再调用 cache clear。这样即使旧 load 与 delete/clear 交错，旧 load 写入前的 generation 校验也会失败。

   备选方案是只调整删除顺序或在写接口后重复失效。该方案不能覆盖已经开始且在失效后完成的 load，无法满足旧 load 回填抑制目标，因此不采用。

3. generation 抑制采用 fail-closed 语义。

   如果 load 因 generation 已过期而被抑制，`RolesForUser` 本次调用返回错误，授权层保持 fail-closed。后续授权请求会重新 miss 并按新 generation 回源最新角色集合。该行为避免在单次并发竞态中错误放行，并与现有“用户角色回源失败时拒绝授权”一致。

   备选方案是在同一次 `RolesForUser` 内自动重试直到 generation 稳定。该方案可能延长授权热路径尾延迟，并在频繁写入时引入复杂重试上限；当前需求只要求旧 load 不写回和后续最终状态可预测，因此优先采用更小实现。

4. cache disabled 模式不参与 generation 门禁。

   当 `rbac.user_role_cache.enabled=false` 时 resolver 不创建 loading cache，`RolesForUser` 每次直接回源并返回独立 slice。失效方法保持安全 no-op 或仅更新内部 generation 而不影响 direct load；回源错误继续 fail-closed，direct stats 仍通过 `LoadSuccess` 与 `LoadError` 表达逐次结果。

   备选方案是在 disabled 模式也强制 generation 检查。该方案不会提升正确性，因为没有缓存写回路径，反而增加不必要状态和测试复杂度，因此不采用。

## Risks / Trade-offs

- [Risk] 被 generation 抑制的 load 会让该次授权请求 fail-closed，短时间内增加 deny/error。→ Mitigation: 仅在真实失效与旧 load 交错时发生；后续请求重新回源并收敛到最新角色集合，测试覆盖错误语义和缓存未污染。
- [Risk] per-user generation map 可能随用户 ID 增长。→ Mitigation: generation 状态只在发生失效的用户上创建；实现时评估与缓存生命周期绑定的清理策略，避免引入跨服务公共 API。
- [Risk] 全量失效与单用户失效交错容易漏校验。→ Mitigation: load token 同时包含全量和 per-user generation；写入前同时校验两者。
- [Risk] 测试并发时序不稳定。→ Mitigation: 使用可控 store/load hook 或同包测试 fixture 阻塞回源，明确构造“load 已开始 -> 失效 -> load 完成”的顺序，并补充 `go test -race`。

## Migration Plan

- 本变更只修改 user-service permission infrastructure 代码和 OpenSpec delta，不需要数据库 migration、OpenAPI 生成、部署清单或配置变更。
- 发布时随普通 HTTP runtime rollout 生效；回滚到旧版本会恢复旧缓存失效行为，不涉及数据回滚。
- 验证应运行相关 permission/role 包测试、包含目标并发测试的 race 测试，以及 `make user-service-architecture-lint`；合并前运行 `make lint` 和 `make verify`。

## Open Questions

- 无待确认问题；实现阶段可在不改变公共 API 的前提下选择 `entUserRoleResolver` 内部字段或同包私有 wrapper 来承载 generation 门禁。
