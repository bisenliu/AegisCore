## Context

role PostgreSQL adapter 当前在业务 mutation 后插入 identity revision 和 outbox event。identity sequence 不参与事务提交排序，因此两个不同行的并发 mutation 可以按 revision 1、2 分配，却按 revision 2、1 提交。permission watcher 和 Casbin engine 把最大可见 revision 当作完整快照水位，旧或相同 revision 又会被快速路径跳过，因而无法发现较小 revision 的晚提交。

在线 command 在提交后同步本地 projection，失败时返回 error。可靠 outbox 能恢复 projection，却无法改变调用方已经收到失败且业务已经提交的事实。绑定 command 还会在提交后重新查询返回集合，存在第二个已提交后失败点。

## Goals / Non-Goals

**Goals:**

- 使任意已提交 revision N 都能证明所有小于 N 的在线 RBAC mutation 已经提交可见。
- 保持 transactional outbox、至少一次发布、Redis max cache 和 Casbin revision gate 的既有边界。
- 让旧、重复或乱序的 `policy_changed` event 仍执行当前权威快照刷新，同时 coalesce 并发刷新。
- 让 API 成败准确表达数据库 mutation 是否提交，并保持同步异常时授权 fail-closed。
- 通过真实 PostgreSQL 并发和故障注入覆盖原验收条件。

**Non-Goals:**

- 不按历史 revision 重建 temporal policy。
- 不引入 MQ、通用 outbox framework、分布式事务或 Redis 权威计数器。
- 不改变 seed、bootstrap 和 migration 的离线同步边界。
- 不改变 HTTP 路由、请求或响应 JSON 结构。

## Decisions

### Decision: 使用事务内单行 counter 分配 commit-ordered revision

新增固定单行 `rbac_policy_revision_counters` 表。每个在线 mutation 在业务变更后、revision/outbox 插入前，对该行执行原子加一并读取更新值。PostgreSQL row update lock 持有到事务结束；后续 mutation 必须等待前一持锁事务提交或回滚后才能获得 revision。因此 revision N+1 不可能先于 revision N 提交，读取最大已提交 revision 可以继续作为完整快照水位。

counter migration 使用现有 `rbac_policy_revisions` 最大值初始化，保证升级后 revision 不倒退。revision 表继续保留 identity 定义以减少破坏性 migration，但在线写必须显式使用 counter 值，不得再依赖 sequence 默认值。

备选方案是 transaction advisory lock。该方案无需 schema，但 Ent transaction 不暴露稳定的标准库 `*sql.Tx` 端口，绕过生成边界会产生脆弱实现。另一个备选是只让 watcher 对旧消息 reload；它能缓解通知路径，却不能让 revision、lag 和 readiness 重新获得完整水位语义，因此不采用。

### Decision: 为全局通知提供强制、可合并的当前快照刷新

`PolicyReloadEngine` 增加语义明确的强制刷新方法。普通 `ReloadToRevision` 继续允许已就绪快速返回；强制刷新即使 target 等于或小于 applied revision 也至少构造一次当前 PostgreSQL snapshot。并发强制刷新复用现有 flight；若 force 在普通候选构造期间到达，flight 必须把 force 保留为 pending 并再次读取数据库，不能把请求到达前的候选冒充强制刷新结果。相同 revision 的强制候选可以替换 enforcer，但较小 revision 始终不能覆盖较大 projection。

watcher 对所有 `policy_changed` 使用强制刷新，对 `user_role_changed` 在数据库 target 不高于 applied 时保持定向 cache invalidation，并在数据库 revision 超前或 projection 未就绪时使用普通追赶 reload。任何 revision gap 都表示中间可能漏收其他用户的绑定事件，追赶完成后必须全量提升 user-role cache generation；周期补偿发现 revision mismatch 时同样执行强制刷新和全量失效。本地提交后的 coordinator 对所有变更推进 projection revision，再按已知变更范围失效缓存，避免等待 Redis 回环期间因 target 超前而持续 fail-closed。

### Decision: 数据库提交结果与 projection 同步结果分离

command 只有在 store transaction 失败时向 API 返回错误。事务已经提交后，本地 reload 或缓存失效失败必须记录 revision、reason 和对象 ID，并依赖已提交 outbox 自动恢复；Casbin engine 的失败状态继续让本实例授权 fail-closed。

Add/Remove 用户角色与角色权限 store 改为在同一事务内返回最终集合，command 不再在提交后进行响应所需数据库查询。这样成功响应准确表示 mutation 已提交，且不会被后置 I/O 改写成失败。

备选方案是新增 `202 committed but pending` 响应或 operation status API。当前同步失败已能通过 readiness、metrics 和 outbox 自动恢复，引入新外部 DTO 和查询生命周期成本更高，因此不采用。

### Decision: 测试必须穿透真实提交和恢复链路

新增 Docker-backed PostgreSQL 测试，通过 transaction barrier 控制两个 mutation 的 revision 分配和提交竞争，证明 counter 锁阻止提交逆序。100 并发测试必须执行真实 role mutation 并检查唯一递增 revision/outbox。Redis 故障恢复和 outbox 重放测试必须组合真实 outbox store、dispatcher、publisher 与至少两个实际 Engine/Watcher projection，不以只返回目标数字的 fake engine 代替授权断言。

## Risks / Trade-offs

- [全局 counter 降低 RBAC 管理写吞吐] -> 在线 RBAC 管理写频率远低于授权读；锁只覆盖单个数据库事务，并通过 100 并发测试记录上界。
- [升级时 counter 初值错误导致 revision 冲突] -> migration 以 `MAX(revision)` 初始化并使用固定主键、非负约束；发布顺序保持 migration 先于新二进制。
- [强制刷新和 revision gap 增加重复读取及全量缓存失效成本] -> 复用 single-flight coalesce；无 gap 的用户事件仍精确失效，正确性优先于漏消息场景的额外全量读取。
- [提交后本地同步失败仍有短暂不可用] -> engine 保持 fail-closed，outbox dispatcher 和数据库周期检查负责自动恢复，API revision 与观测指标提供诊断依据。

## Migration Plan

1. 生成并审查 counter Ent schema 与 Atlas migration；migration 以现有最大 revision seed 固定行。
2. 在 HTTP rollout 前受控执行 migration，再发布同时使用 counter 的新二进制。
3. 观察 outbox backlog、policy reload lag、readiness 和同步失败指标，确认无 revision 冲突或持续 lag。
4. 回滚应用时保留新增 counter 表；旧二进制仍可使用 identity sequence，但再次前滚前必须将 counter 与 revision 最大值重新对齐，因此不建议混合版本长期并行写入。

## Verification

- 运行 OpenSpec、Ent generate、Atlas migration validate 和 OpenAPI drift 检查。
- 运行 role/permission 单元测试与 `-race`。
- 启用 `-aegiscore.testcontainers` 运行真实 PostgreSQL 并发、outbox 和 e2e 测试。
- 运行 `make user-service-architecture-lint`、`make lint` 和 `make verify`。

## Open Questions

无。
