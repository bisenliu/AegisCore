## Context

前置 change `add-rbac-policy-revision-outbox` 已让每次在线 RBAC mutation 与单调数据库 `policy_revision` 原子提交，`add-rbac-policy-outbox-dispatcher` 则负责至少一次传播该 revision。当前消费端仍存在语义断层：Casbin loader 只返回规则，`Engine.Reload(ctx)` 在锁外构造 enforcer 后无条件替换，watcher 再将消息 revision 写入独立 `VersionTracker`。因此消息处理顺序、tracker 数值与 enforcer 实际来源快照可以不一致；两个并发 reload 也可能由较慢的旧构造覆盖较新的 enforcer。

本 change 位于 `user-service/internal/features/permission/`，影响 Casbin policy loader/engine、application reload port、coordinator、Redis watcher、status/health 和 feature metrics。数据库 relation 与 revision 是权威来源，Redis 只提供 target revision；HTTP API、OpenAPI、数据库 schema、outbox dispatcher、部署清单和 `common/` primitive 不变。

## Goals / Non-Goals

**Goals:**

- 将每个候选 Casbin enforcer 与同一 PostgreSQL 一致性快照中的 `policy_revision` 绑定为 `PolicySet`。
- 保证成功加载的 `PolicySet.Revision >= targetRevision`，且任何候选投影都不能使 engine applied revision 倒退。
- 让 engine 状态成为 applied revision、最近 reload error、reload status、startup/readiness 和 reload lag 的唯一事实源。
- 合并同实例并发 reload 的 target，以有界构造次数最终应用请求期间观察到的最高数据库 revision。
- 明确定义 reload 失败时继续持有上一成功 enforcer，但状态不可用、applied revision 不推进且授权 fail-closed。
- 以可控乱序和 100 并发测试证明交换、coalesce、状态和观测语义。

**Non-Goals:**

- 不创建或修改 policy revision/outbox schema，不实现 outbox dispatcher、claim、publish 或 retry。
- 不处理 user-role cache inflight load 在 invalidation 后回填旧值的问题，也不重做 cache generation 协议。
- 不按历史 revision 重建任意旧 policy；target 是最低可接受 revision，loader 可返回更高的当前快照。
- 不保留 `Reload(ctx)`、通知序号 tracker 或无 revision 消息的兼容路径。
- 不改变 HTTP DTO、路由、OpenAPI、Casbin model、权限规则形状或授权热路径；热路径不读取 PostgreSQL/Redis revision。

## Decisions

### 1. PolicySet 在同一 PostgreSQL snapshot 中绑定 revision 与规则

`PolicySet` 增加 `Revision int64`，loader port 改为 `LoadPoliciesAtLeast(ctx, targetRevision) (PolicySet, error)` 或等价命名。每次加载在只读、`REPEATABLE READ` PostgreSQL transaction 中先读取该 snapshot 可见的最新 `rbac_policy_revisions.revision`，再读取角色权限规则；revision 与 mutation 在前置 change 中同 transaction 提交，因此随后规则查询对应同一个已提交授权快照。超级管理员 wildcard 仍由 `rbacbaseline` 在内存追加，但被视为该 `PolicySet` 的固定组成。

若 snapshot 最新 revision 小于 target，loader 必须结束该 transaction，并在 context deadline 内以有界退避重新打开新 snapshot；不得在旧 snapshot 内等待后继续读取，也不得返回低于 target 的规则。target 为 0 用于启动初始化，表示加载当前可见 latest revision；没有任何 revision 记录时以 revision 0 构造基线投影。context 取消、revision 查询失败、规则查询失败或 transaction 完成失败均返回 error，不产生可交换候选。

备选方案是在 auto-commit 下分别查询 revision 和规则，但两个语句可能跨越 mutation commit，无法证明绑定关系；按 target 查询历史规则则需要 temporal schema，超出当前数据模型；只在加载结束后读 latest revision 会给可能更旧的规则错误盖上新 revision，均不采用。

### 2. Engine 原子拥有 enforcer、applied revision 与 reload 状态

engine 状态在同一锁下保存 `enforcer`、`appliedRevision`、`lastErr` 和最近 reload 状态。`ReloadToRevision(ctx, targetRevision)` 先获得 `PolicySet` 并在锁外完成 model/enforcer 构造，再在写锁内比较候选 revision：只有候选 revision 大于当前 applied revision 时才同时交换 enforcer 与 revision；等于当前 revision 视为幂等成功且不重复交换；小于当前 revision 作为 stale candidate 丢弃，绝不能覆盖当前 enforcer或降低 revision。

成功应用或确认当前投影已不低于 target 时清除 reload error 并记录成功；加载、构造或达到 target 失败时保留现有 enforcer/applied revision、记录最近 error 和失败指标。为保持现有安全契约，`Enforce` 在 last reload status 为失败时即使存在旧 enforcer也必须 fail-closed；成功追平后才恢复授权与 startup/readiness。这样“保留上一成功 revision”只用于恢复和诊断，不表示失败期间继续放行业务请求。

engine 暴露只读 `AppliedRevision()` 与状态 snapshot；删除可被独立推进的 `VersionTracker.MarkApplied`，或将 tracker 收窄为直接读取同一 engine 状态的 facade。watcher、health、metrics 和 coordinator 不得维护第二份“已应用”数值。备选方案是保留 atomic tracker 并在 reload 后更新，但交换成功与 tracker 更新之间仍有可见窗口且未来调用方可再次错误推进，故不采用。

### 3. 使用 max-target single-flight coalescing，并保留锁内防倒退

同一 engine 同时只允许一个 reload leader 执行数据库加载和 enforcer 构造。其他调用把 `pendingTarget` 原子提升为 `max(current, target)` 并等待共享结果；leader 每轮读取最高 pending target，加载至少该 revision 的候选并应用，若构造期间出现更高 target则继续下一轮，直到 applied revision 不低于已观察最高 target或本轮失败。等待方只在自身 target 已被实际 applied 时成功；context 取消只取消该等待方，不能取消其他调用所需的共享 reload。

初始实现应使用 engine 内最小 mutex/条件变量或完成 channel 状态机，不引入通用 scheduler、workerpool 或后台常驻 goroutine。即使后续重构绕过 coalesce，候选交换仍执行锁内 revision 比较，因此受控乱序构造也不能回退投影。100 个并发 target 最终只要求 applied 等于加载时数据库 latest 且不低于最高 target，不要求 revision 连续，也不要求固定构造次数。

备选方案是仅用一个 mutex 串行执行全部 100 次 reload，正确但制造无意义数据库负载；允许全部并发再仅靠交换 CAS 虽防倒退，却放大连接和 CPU 使用；常驻 worker 增加 lifecycle 复杂度，均不采用。

### 4. application port 以数据库 revision 表达目标与完成结果

permission application 的 reload port 改为 revision-aware 方法，并返回实际 applied revision或可读取的状态。coordinator 在本实例 mutation 后使用 transaction 返回的数据库 revision作为 target；watcher 在 Pub/Sub 和周期补偿路径使用消息/Redis 中的数据库 revision作为 target。通知 arrival、Redis max revision、reload attempt 和 applied revision是不同状态，只有 engine 成功应用或确认等价投影后才能推进 applied。

对于 `policy_changed`，watcher 调用 `ReloadToRevision`，成功后再执行既有全量 user-role cache invalidation。对于只需要定向失效的 `user_role_changed`，保持前置 dispatcher 的幂等 invalidation 行为，但该通知不得直接冒充 Casbin engine applied revision；周期补偿或下一次 policy reload仍以数据库 latest target校准 engine。由于本 change不解决 inflight cache回填，相关限制继续由既有 cache安全语义和后续独立 change处理。

Pub/Sub 重复和乱序消息继续安全：低于 engine applied revision 的 policy target是幂等成功但消息要求的 cache side effect仍按 kind执行；较高 target加入 coalesce。备选方案是 watcher 在调用 engine 前根据独立 tracker跳过消息，会重新引入状态分裂并可能遗漏定向 side effect，故不采用。

### 5. lag、metrics 与 health 只描述真实投影

`local_applied_policy_revision` 一律来自 engine；`remote/latest` 来自 Redis revision cache或数据库可见 latest，lag 定义为 `max(knownLatest - engine.AppliedRevision(), 0)`。lag 为 0 只表示在当前已知 latest 下 engine投影不落后，不能由消息接收或失败 attempt清零。reload成功指标记录实际候选 revision，失败不得改变 applied gauge；标签保持低基数，不增加 revision、user、role、permission或错误文本标签。

startup/readiness 要求 engine 已有成功初始化投影、最近 reload状态成功，且在存在已知 target时 applied revision不低于该 target。reload失败、target未追平或 engine未加载均拒绝业务流量；watcher/Redis未知状态继续通过各自 health信号表达，不伪造 lag=0。日志可用字段记录 `target_revision`、`candidate_revision`、`applied_revision` 和稳定错误类别，但不得记录 policy内容、SQL或Redis key。

备选方案是 lag继续读取 watcher tracker，无法证明与 enforcer一致；失败后仍允许旧 enforcer会让撤权变更处于 fail-open窗口，均不采用。

### 6. 代码归属与外部边界保持不变

revision-aware loader和engine留在 `user-service/internal/features/permission/infrastructure/casbin/`，消费侧 reload/status port留在 permission application，Redis watcher只负责通知适配，Fx named resource与lifecycle wiring留在 permission composition。数据库 snapshot使用现有共享 Ent client/driver，但 RBAC `PolicySet`、target与revision状态不得进入 `common/`、`internal/shared/`或`internal/integration/`。

`deployments/`无需变更；若现有 dashboard/alert查询的指标名称不变，仅修正代码语义和测试，不生成观测资产 diff。`docs/openspec`通过本 delta描述行为，归档后合并到 `rbac-access-control`主规格。不存在 Ent schema、Atlas migration、OpenAPI或服务间协议变更。

## Risks / Trade-offs

- [目标 revision 已发布但当前数据库 replica 尚不可见] → loader以新 snapshot有界重试并服从 context deadline；不返回低 revision，不无限占用旧 transaction。
- [reload失败期间保留旧 enforcer可能被误用] → engine状态与 enforcer一起检查，最近 reload失败时授权和 readiness均 fail-closed，applied revision只用于诊断与后续恢复。
- [coalesce leader的调用 context过早取消] → shared reload使用由 engine管理且受启动/调用总期限约束的执行 context，单个 waiter取消只退出自身等待；实现不得创建无期限后台任务。
- [持续到达更高 revision导致 leader循环过久] → 每轮直接加载当前 snapshot latest，通常一次跨越多个 target；context和数据库 timeout提供上界，失败保持明确状态并由后续通知/补偿重试。
- [定向 user-role invalidation与全局 Casbin applied revision不是同一投影维度] → applied明确表示 Casbin policy projection；定向 side effect不得推进它，既有 user-role cache状态单独保持fail-closed语义，inflight修复留给独立 change。
- [从独立 tracker迁移可能影响 metrics/health测试] → 保留只读接口形状时只能委托engine状态，不允许第二份可写值；通过跨组件一致性测试覆盖。
- [受控乱序测试需要构造调度点] → 在现有 loader test double中使用channel控制返回顺序，不为测试向生产代码加入无业务逻辑的hook。

## Migration Plan

1. 先改造 loader与测试，使其从同一只读 snapshot返回 `PolicySet{Revision, PermissionRules}`并实现target等待语义。
2. 改造 engine状态、revision-aware reload、coalesce与防倒退交换，删除无revision reload入口；同步更新authorization fail-closed检查。
3. 改造 application port、coordinator、watcher、composition、health与metrics，使数据库revision贯穿target且engine成为applied唯一来源。
4. 运行可控乱序、100并发、失败恢复、race-sensitive、lag与readiness测试，再运行architecture lint、lint和verify。
5. 本 change无数据库或外部协议rollout顺序；部署时应确保前置revision/outbox与dispatcher change已完成，再滚动全部副本。混合版本不提供旧reload兼容，必须按受控发布窗口完成。

回滚应用时回退整个消费端二进制即可，不修改数据库revision/outbox数据或Redis消息；不得仅回退engine而保留调用新revision port的watcher。失败实例继续readiness=false，由负载均衡摘除。

## Verification

- 运行 Casbin loader PostgreSQL测试，证明revision与规则来自同一snapshot、低于target时重开transaction、context取消与数据库错误不返回候选。
- 运行 engine单元与 `-race` 测试，覆盖 revision 1晚于revision 2完成、stale/equal候选、100并发target、waiter取消、失败不推进与成功恢复。
- 运行 watcher/coordinator测试，覆盖重复/乱序数据库revision、定向cache side effect、周期补偿以及不存在独立tracker推进路径。
- 运行 authorization、metrics、router health/startup测试，证明last error时fail-closed、applied与实际engine一致、lag不被失败清零且lag为0表示不落后。
- 运行相关 Go package测试和 `make user-service-architecture-lint`；确认无Ent生成物、migration、OpenAPI或deployments非预期diff，最终按仓库流程运行 `make lint`与`make verify`。

## Open Questions

无。
