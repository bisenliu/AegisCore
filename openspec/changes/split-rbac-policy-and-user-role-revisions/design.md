## Context

RBAC 当前使用单一 `rbac_policy_revision` 表示角色权限策略变更和用户角色绑定变更。角色侧用户角色写入在同一事务中递增该 revision、写入 outbox event，并在提交后通知 permission coordinator；permission coordinator 对所有 change 都先调用 `ReloadToRevision`，导致纯 `user_role` 变更也会重建 Casbin enforcer。Watcher 收到 Pub/Sub payload 后逐条读取 PostgreSQL latest revision 并处理，policy 消息会强制 `RefreshToRevision`，重复投递会重复全量加载。

本 change 影响 `role` 和 `permission` 两个 feature 的同步边界、PostgreSQL schema、outbox envelope、Redis Pub/Sub 消费、Casbin engine 调用关系和 RBAC 观测。变更不进入 `common`，不新增 eventbus/MQ，不把 RBAC 业务 revision 或 envelope 下沉到共享 primitive。

## Goals / Non-Goals

**Goals:**

- 将 Casbin 静态 policy revision 与用户角色缓存失效 revision 分离，使纯用户角色绑定变更不触发 `LoadPoliciesAtLeast`、不扫描 `role_permissions` 全集、不推进 Casbin engine applied revision。
- 让 watcher 合并待处理通知，对同批 policy 通知只按最高未应用 policy revision 重建一次，对重复、相等或乱序通知保持幂等。
- 保持用户角色缓存失效的同步语义：精确用户事件失效单用户缓存，无法证明精确集合完整时失效全部用户角色缓存。
- 保持 fail-closed：policy 未 ready 或用户角色回源失败仍拒绝授权，不能用旧投影放行请求。
- 同步更新测试、metrics、日志、dashboard/runbook 和 OpenSpec 主规格语义。

**Non-Goals:**

- 不改变权限、角色、用户角色 HTTP API 路径、请求和响应契约。
- 不新增外部 MQ、eventbus、gRPC 或跨服务 RBAC 协议。
- 不把 RBAC revision、outbox envelope、user-role cache key schema 放入 `common`。
- 不保留旧 Redis payload、旧 outbox kind 或旧单 revision 消费兼容分支。

## Decisions

### 决策 1：拆分持久化 revision

纯 policy 变更继续写入 `rbac_policy_revision`，只覆盖角色状态、角色权限绑定、权限投影等会改变 Casbin 静态规则集合的事实。用户角色绑定变更写入新的 user-role revision 持久化结构，例如 `rbac_user_role_revision`，并创建 `user_role_changed` outbox event。

备选方案是继续使用单一 revision 但在消费端按 kind 判断是否 reload。该方案会让 PostgreSQL latest policy revision 继续被 user-role 写入推进，导致 lag、readiness、周期补偿和 applied revision 语义继续混乱，因此不采用。

### 决策 2：拆分 application 通知路径

permission application 暴露明确的 policy change 与 user-role change 处理语义。Policy change 必须执行 revision-aware reload 并验证 projection status；user-role change 不调用 `ReloadToRevision` 或 `RefreshToRevision`，只执行用户角色缓存失效。

备选方案是在现有 `PolicyChange` 上保留 `Kind` 分支并传入同一 revision。该方案仍允许调用方误传 policy revision 目标，且测试无法强约束纯 user-role 变更不影响 Casbin applied revision，因此不采用。

### 决策 3：Watcher 使用批处理合并

Watcher 主循环在收到消息后 drain 当前可用消息形成 bounded batch，按 event kind 聚合副作用：policy 事件取最高 policy revision，user-role 事件聚合 user ID。若 batch 中存在 policy target 高于当前 applied 或 projection 不 ready，则最多执行一次 reload；若 policy target 已应用且 ready，则跳过 reload。User-role 事件始终执行缓存失效，存在 gap 或无法精确枚举用户时执行全量用户角色缓存失效。

备选方案是仅依赖 engine 内部 reload flight 合并。该机制只合并并发调用，不能合并 watcher 串行 backlog，也不能跳过重复 policy 强制 refresh，因此不采用。

### 决策 4：Outbox envelope 明确字段语义

`policy_changed` payload 携带 `policy_revision`，`user_role_changed` payload 携带 `user_role_revision` 和 `user_id`。两类事件使用各自 revision 生成幂等键。消费者拒绝缺少必要字段或 kind/revision 不匹配的 payload，不保留旧 envelope 兼容解析。

备选方案是保留 `policy_revision` 字段并对 user-role 事件重命名含义。该方案会继续让字段名与行为相悖，增加误用风险，因此不采用。

### 决策 5：观测只把 policy lag 绑定到 policy revision

`rbac_policy_reload_lag` 继续表示 PostgreSQL latest policy revision 与 Casbin engine actual applied revision 的非负差值。User-role revision 不进入该指标；user-role 事件只通过 dispatcher/watcher kind 计数、日志和缓存失效测试体现。

备选方案是新增混合 lag。混合 lag 无法直接表示 Casbin policy 新鲜度，且会误导 readiness 判断，因此不采用。

## Risks / Trade-offs

- [Risk] 数据库 migration 会改变 outbox/revision 写入结构，部署顺序错误可能导致旧副本无法消费新 payload。→ Mitigation：不保留兼容分支，发布顺序必须先 migration，再滚动所有 user-service 副本，变更窗口内避免混跑旧新应用。
- [Risk] User-role 事件丢失后精确缓存失效可能不完整。→ Mitigation：watcher 在发现无法证明精确集合完整时失效全部用户角色缓存，TTL 和回源 fail-closed 继续兜底。
- [Risk] 批处理 coalesce 可能延迟单条消息处理。→ Mitigation：只 drain 当前已到达消息，不引入长等待窗口；保持周期性 PostgreSQL policy revision 补偿。
- [Risk] 指标和 dashboard 文案不更新会误读 user-role revision。→ Mitigation：同一 change 更新 metrics 测试、dashboard/provisioning、alert annotation 和 runbook 文案。

## Migration Plan

1. 更新 Ent schema，新增 user-role revision 持久化结构或等价提交水位，并调整 outbox event schema 以表达 policy/user-role 分离字段。
2. 生成 Ent 代码和 Atlas SQL migration，验证 migration 可应用。
3. 修改 role PostgreSQL store：policy 写入只创建 policy revision，user-role 写入只创建 user-role revision 和对应 outbox event。
4. 修改 permission application、Redis message、watcher 和 dispatcher 消费语义。
5. 更新观测、dashboard、测试和 OpenSpec 文档。
6. 发布时先执行 migration，再执行 RBAC seed，最后滚动 user-service 副本；不得混跑旧新 envelope 消费者。

Rollback 策略：如果 implementation 尚未发布，直接撤回代码和 migration。若 migration 已执行且新副本已发布，回滚必须通过新的受控 migration 和应用回滚计划恢复旧 schema；本 change 不提供运行时兼容开关。

## Open Questions

无。
