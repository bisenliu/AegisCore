## ADDED Requirements

### Requirement: RBAC policy revision 提交顺序水位

系统 MUST 使在线 RBAC policy revision 的数值顺序与事务提交可见顺序一致；任一已提交 revision N MUST 表示所有已分配且小于 N 的在线 mutation 已经提交或回滚，数据库最大已提交 revision 才能作为完整授权快照水位。revision 分配、业务 mutation 和 outbox event MUST 保持同一 PostgreSQL transaction，Redis 或进程内状态 MUST NOT 参与权威 revision 分配。

#### Scenario: 并发 mutation 不得按 revision 逆序提交

- **WHEN** 两个在线 RBAC mutation 并发执行且较早 mutation 已获得 revision N 但尚未提交
- **THEN** 后续 mutation MUST NOT 以 revision N+1 先行提交
- **AND** 后续 mutation MUST 等待前一事务提交或回滚后再获得可提交 revision

#### Scenario: 升级后 revision 从已有最大值继续

- **WHEN** 数据库已经存在 policy revision 并应用 revision counter migration
- **THEN** counter MUST 以当前最大已提交 revision 初始化
- **AND** 新在线 mutation 分配的 revision MUST 大于全部已有 revision

### Requirement: 全局 policy 通知刷新当前权威快照

系统 MUST 对每条有效 `policy_changed` 通知执行当前 PostgreSQL 权威 policy 快照刷新。刷新 MAY 与同时发生的刷新合并，但 MUST NOT 仅因通知 revision 小于或等于 applied revision 而跳过；候选快照低于当前 target 时 MUST NOT 交换，相同 revision 的强制刷新候选 MUST 能更新其绑定的 enforcer。

#### Scenario: 较小 revision 通知晚到

- **WHEN** 实例已应用较大 revision 后收到较小 revision 的 `policy_changed` 通知
- **THEN** 实例 MUST 至少重新读取并应用一次当前 PostgreSQL 权威快照
- **AND** applied revision MUST NOT 倒退，旧候选 MUST NOT 覆盖较新候选

#### Scenario: 重复全局通知被合并

- **WHEN** 多条重复或乱序 `policy_changed` 通知并发触发刷新
- **THEN** engine MAY coalesce 同一时刻的刷新请求
- **AND** 所有调用完成时实际 enforcer MUST 对应不低于最高 target 的当前权威快照

#### Scenario: 强制刷新加入正在构造的普通 reload

- **WHEN** 强制刷新请求在普通 reload 已开始读取数据库后加入同一 flight
- **THEN** engine MUST 在该强制请求之后重新读取一次 PostgreSQL 快照
- **AND** MUST NOT 把强制请求到达前构造的候选视为该请求已经完成

### Requirement: revision gap 恢复全部用户角色缓存

系统 MUST 在 watcher 从较低 applied revision 直接追赶到较高数据库 revision 时，全量提升本实例 user-role cache generation；当前消息中的精确 user ID 不足以证明中间 revision 未包含其他用户绑定变更。仅当数据库 target 不高于 applied revision 时，重复 `user_role_changed` event MAY 只失效消息指定用户。

#### Scenario: 漏收前序用户绑定通知后收到更高 revision

- **WHEN** 实例漏收用户 A 的 `user_role_changed` event，随后收到用户 B 对应的更高数据库 revision
- **THEN** watcher MUST 追赶到数据库 revision 并失效全部 user-role cache
- **AND** 用户 A 的旧缓存 MUST NOT 因后续消息只包含用户 B 而永久保留

### Requirement: RBAC 写 API 准确表达数据库提交结果

在线 RBAC 写 API MUST 仅以业务 mutation transaction 是否提交决定 mutation 成败。transaction 提交后发生的本地 reload 或缓存失效错误 MUST 保持实例 fail-closed、记录已提交 revision 并由 outbox 自动恢复，但 MUST NOT 把已提交 mutation 向调用方表达为失败；成功响应所需数据 MUST 在提交 transaction 内产生或无需提交后数据库读取。

#### Scenario: 提交后本地 reload 失败

- **WHEN** RBAC mutation、revision 和 outbox 已提交，但本地 policy reload 失败
- **THEN** API MUST 返回该 mutation 的成功结果
- **AND** 本实例授权 MUST fail-closed，pending outbox MUST 保持可投递并在后台恢复 projection

#### Scenario: 提交前任一步失败

- **WHEN** 业务 mutation、revision counter、revision、outbox 或 transaction commit 任一步失败
- **THEN** API MUST 返回失败并且 transaction 内全部变化 MUST 回滚
- **AND** command MUST NOT执行提交后本地同步

#### Scenario: 绑定写响应不执行提交后查询

- **WHEN** 用户角色或角色权限 Add、Remove 或 Replace transaction 成功
- **THEN** store MUST 返回同一 transaction 内构造的最终绑定集合与 committed revision
- **AND** command MUST NOT 为构造成功响应在 commit 后重新查询数据库

### Requirement: RBAC 并发与故障验收覆盖真实链路

系统 MUST 使用真实 PostgreSQL transaction 验证 revision 提交顺序和 100 并发 mutation，并使用真实 outbox store、dispatcher、Redis publisher、watcher 与 Casbin engine 验证 Redis 故障恢复、重放和多副本最终授权收敛；仅并发调用 fake loader 或手工推进 fake revision 的测试 MUST NOT 作为这些验收场景的唯一证据。

#### Scenario: 一百个并发写最终收敛

- **WHEN** 100 个在线 RBAC mutation 并发提交，并对投递链路执行独立的可控 Redis publish 故障验收
- **THEN** 每个成功 mutation MUST 具有唯一 commit-ordered revision 和 pending outbox event
- **AND** Redis 恢复后无需新增 mutation，所有测试副本 MUST 最终应用数据库最大 revision 且授权结果对应最终关系数据

#### Scenario: Add Remove Replace 重放

- **WHEN** Add、Remove 和 Replace event 因 publish 后 ack 前故障被重复投递
- **THEN** dispatcher MUST 最终 ack 每个 event，watcher 副作用 MUST 保持幂等且不得丢失必要刷新或缓存失效
