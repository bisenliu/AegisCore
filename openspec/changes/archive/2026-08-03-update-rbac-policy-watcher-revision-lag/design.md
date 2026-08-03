## Context

前置 change 已建立 PostgreSQL 单调 policy revision、outbox dispatcher 和 revision-aware Casbin projection：engine 的 `AppliedRevision()` 表示实际成功应用的数据库授权快照，并通过 apply gate 防止旧候选覆盖新投影。当前 `user-service/internal/features/permission/infrastructure/redis/watcher.go` 仍在 `CheckVersion` 中调用 Redis `CurrentVersion()`，并以该值与 applied revision 计算 mismatch 和 lag；Pub/Sub payload 的 revision 也会直接成为 reload target。

这使 Redis 同时承担唤醒通道和补偿权威两个角色。Pub/Sub 丢失且 Redis counter 落后、Redis 数据被重建或 counter 不存在时，周期检查无法发现 PostgreSQL 已提交的新 revision；反之，旧消息也不能证明数据库 latest 或本地投影状态。最终可能出现 lag 为 `0`、watcher 无 mismatch，但 Casbin projection 仍落后于数据库的假收敛。

本 change 影响 `user-service/internal/features/permission/` 的 application port、PostgreSQL adapter、Redis watcher、composition、metrics、日志和测试，以及 `deployments/observability/`、`deployments/compose/grafana/` 与 `docs/observability/`。不改变 HTTP API、OpenAPI、数据库 schema、Redis key/channel schema、Casbin engine apply gate或授权热路径。

## Goals / Non-Goals

**Goals:**

- 让数据库 latest policy revision 成为 watcher 周期补偿和 reload lag 的唯一权威远端值。
- 让 Redis Pub/Sub 仅作为低延迟唤醒 hint；消息丢失、重复、乱序和旧 revision 均不破坏最终收敛或使投影倒退。
- 保证 lag 始终按 `max(databaseLatest - localApplied, 0)` 计算，lag 为 `0` 时本地 applied projection 不落后于本次成功读取的数据库 latest revision。
- 区分数据库 revision source 不可用、数据库 revision mismatch、reload 失败和 reload 成功的低基数 metrics reason 与日志字段。
- 同步更新 dashboard、alert、runbook、生成物和测试 fixture。

**Non-Goals:**

- 不新增或修改 policy revision/outbox Ent schema、Atlas migration，也不实现 outbox dispatcher。
- 不修改 Casbin engine 的 revision-aware loader、并发 coalesce 或防倒退 apply gate；本 change 依赖这些前置能力。
- 不改变用户角色 cache generation、inflight refill 或定向 invalidation 语义。
- 不在授权请求热路径读取 PostgreSQL 或 Redis revision。
- 不保留 Redis version lag、双写指标或兼容 PromQL。

## Decisions

### 1. permission application 拥有 latest revision 查询端口

在 permission application 边界声明只读、最小的 `LatestPolicyRevisionSource` 或等价接口，只暴露 `LatestPolicyRevision(ctx) (int64, error)`。PostgreSQL adapter 使用现有 Ent client 查询 `rbac_policy_revisions` 的最大 revision；空表返回 revision `0`，查询失败保留底层 cause 并由 watcher 映射为稳定诊断原因。该读取不需要新 schema、migration、锁或 transaction，因为 watcher 只需要一次已提交 latest revision 快照作为 reload 最低目标。

端口和 revision 业务语义留在 `user-service/internal/features/permission/`；Ent 查询实现留在 permission infrastructure，Fx named `primary_db` 选择和生命周期接线留在 composition。不得将接口或实现放入 `common/`、`internal/shared/` 或 `internal/integration/`，也不得让 application 导入 Ent predicate/concrete client。

备选方案是复用 Redis `CurrentVersion()`，但 Redis 可丢失或重建，无法证明数据库提交事实；另一个方案是让 watcher调用 policy loader只为获得 revision，会耦合昂贵规则加载并模糊 reload职责，均不采用。

### 2. Pub/Sub 与周期检查统一先校准数据库 latest

`CheckVersion` 每个周期直接查询 latest revision source，成功后以数据库 latest 与 engine applied revision判断是否需要 `ReloadToRevision`。`HandlePayload` 成功解析消息后也查询数据库 latest；payload revision、instance ID 和 reason仅用于低风险诊断和决定既有 cache side effect类型，不得直接推进 applied revision、清零 lag或覆盖数据库目标。

当 payload revision高于本次数据库可见 latest时，以数据库 latest为当前目标，不等待 Redis值成为权威；后续周期检查继续校准。当前置 dispatcher已发布、读取连接短暂尚不可见目标时，revision-aware loader和后续周期检查负责最终追平。重复、乱序或旧消息会触发至多一次幂等数据库校准，engine apply gate继续保证投影不倒退。

备选方案是仅在 payload revision高于local时查询数据库，可被旧消息或无消息跳过；另一个方案是使用`max(payload, databaseLatest)`，会重新让不可信消息制造数据库尚不可证明的target，均不采用。

### 3. 数据库查询失败保持状态未知且不伪造收敛

latest revision source不可用时，watcher记录稳定的`revision_store_unavailable`或等价低基数reason及`source="watcher_pubsub|watcher_periodic_check"`，日志使用`hint_revision`、`local_applied_policy_revision`、`source`和稳定error category。该路径不得调用以Redis counter为target的reload，不得把lag设为`0`，也不得记录reload success。

是否将单次数据库查询失败直接并入watcher readiness沿用现有health contract，不在本change引入新的独立health component；持续查询失败由check failure指标、Redis/DB health与lag/未收敛告警共同暴露。备选方案是失败时沿用最近数据库latest驱动reload，但可能在进程重启后没有可信快照；因此仅保留上次lag观测值，不把缓存值冒充当前数据库latest。

### 4. lag只由database latest和engine applied计算

`aegiscore_user_service_rbac_policy_reload_lag`保留现有指标名称，但稳定语义改为`max(database_latest_policy_revision - local_applied_policy_revision, 0)`。数据库查询成功后立即观察一次lag；reload成功后使用同一次已知database latest与engine返回/状态中的actual applied revision再次观察。reload失败保留非零lag；若 applied已高于读取到的latest则按非负规则输出`0`。

lag gauge不增加revision、user、role、permission、Redis key、raw error等label。Redis counter/latest、payload revision或消息处理成功均不得单独更新lag。dashboard标题、说明、PromQL fixture、alert annotation和runbook明确database/local语义，并删除旧Redis/local差值描述；不保留第二个旧指标。

备选方案是分别导出database latest和local applied gauge再由PromQL计算，但会扩大稳定指标面并使跨scrape时刻差值产生歧义；继续由feature recorder原子计算单值lag更符合现有契约。

### 5. metrics reason与日志字段表达事实来源

watcher mismatch只在成功读取database latest且`databaseLatest > localApplied`，或engine projection状态未ready时记录。check/reload指标使用固定枚举区分`revision_store_unavailable`、`revision_mismatch`、`reload_failed`和成功；删除或重命名仍表达Redis version store权威性的reason。日志统一使用`database_latest_policy_revision`、`local_applied_policy_revision`、`target_revision`、`hint_revision`、`source`和稳定reason，不再使用含混的`remote_policy_revision`、`remote_version`或`version_check`表达数据库事实。

这些字段可记录数值revision，但metrics label不得包含revision或原始错误。备选方案是保留旧字段以兼容日志查询，但会延续错误语义且用户明确不要求兼容，故不采用。

## Risks / Trade-offs

- [周期检查增加PostgreSQL查询] → 查询仅取单行最大revision并复用现有索引/主键；保持现有有界check interval和context timeout，不在授权热路径执行。
- [Pub/Sub突发导致数据库校准查询放大] → watcher当前串行消费消息，engine reload具备coalesce；实现可在不改变正确性的前提下合并同一消费循环中的唤醒，但不得使用Redis revision跳过数据库校准。
- [数据库查询失败期间lag可能陈旧] → 保留上一观测值并同时增加revision store unavailable计数/日志，禁止清零；DB health和持续失败告警用于表达未知状态。
- [数据库latest在读取后继续前进] → lag是scrape前最近一次成功校准快照；下一条hint或周期检查重新读取，最终一致SLO由周期上界和alert覆盖。
- [旧应用与新dashboard短暂混部] → 指标名不变但语义改变，部署窗口内dashboard可能混合两种语义；采用受控滚动发布并先更新应用、后启用新annotation，不提供长期双版本查询。
- [前置projection change未完成] → apply前确认engine的applied revision来自真实投影且apply gate防倒退；否则不得实现本change或宣称lag为0具备安全语义。

## Migration Plan

1. 确认前置policy revision/outbox、dispatcher和revision-aware Casbin projection change已完成，engine applied revision是实际投影唯一来源。
2. 增加permission-owned latest revision query port、PostgreSQL adapter和composition接线，再切换watcher的Pub/Sub与周期检查路径。
3. 更新metrics reason、日志字段、lag recorder及单元/集成测试，删除Redis counter权威判断与旧fixture。
4. 更新Prometheus rules、Grafana dashboard源、Compose provisioning生成物和runbook，并执行dashboard drift校验。
5. 先滚动发布应用实例，再同步启用更新后的dashboard/alert说明；无需数据库migration、OpenAPI生成或Redis数据迁移。

回滚时整体回退应用和观测资产到上一版本；现有数据库revision/outbox与Redis消息无需清理。不得只回退watcher而保留database-lag说明，也不得在混合版本期间把旧Redis lag解释为数据库收敛证明。

## Verification

- 运行permission watcher与PostgreSQL adapter测试，覆盖空revision表、database latest超前、store error、context取消、Pub/Sub丢失/重复/乱序/旧消息和Redis counter缺失/落后/重建。
- 运行revision-aware engine/watcher集成测试，覆盖reload失败不推进applied且lag不清零、后续周期检查恢复并最终lag为0。
- 运行permission metrics测试，校验reason/source allowlist、非负lag和日志字段，不出现旧Redis version语义或高基数label。
- 运行`make user-service-architecture-lint`、`make compose-dashboard-check`及相关Prometheus/dashboard fixture校验。
- 完成本change的代码、规格和文档后暂存预期变更，再运行`make lint`和`make verify`。

## Open Questions

- 无。latest revision source、lag公式和Redis hint边界均由本change确定。
