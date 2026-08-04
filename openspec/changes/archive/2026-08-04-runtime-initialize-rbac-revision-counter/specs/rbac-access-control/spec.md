## MODIFIED Requirements

### Requirement: RBAC policy revision 提交顺序水位

系统 MUST 使在线 RBAC policy revision 的数值顺序与事务提交可见顺序一致；任一已提交 revision N MUST 表示所有已分配且小于 N 的在线 mutation 已经提交或回滚，数据库最大已提交 revision 才能作为完整授权快照水位。revision counter 初始化与分配、业务 mutation 和 outbox event MUST 保持同一 PostgreSQL transaction，Redis、进程内状态或 migration seed DML MUST NOT 参与权威 revision 分配。

#### Scenario: 并发 mutation 不得按 revision 逆序提交

- **WHEN** 两个在线 RBAC mutation 并发执行且较早 mutation 已获得 revision N 但尚未提交
- **THEN** 后续 mutation MUST NOT 以 revision N+1 先行提交
- **AND** 后续 mutation MUST 等待前一事务提交或回滚后再获得可提交 revision

#### Scenario: 已有 counter 直接原子递增

- **WHEN** 固定 revision counter 行已经存在且在线 RBAC mutation 分配新 revision
- **THEN** transaction MUST 原子递增 counter 并使用返回值写入 revision 与 outbox
- **AND** 正常路径 MUST NOT 为初始化重复读取最大 revision 或重建 counter

#### Scenario: counter 缺失时从已有最大 revision 幂等初始化

- **WHEN** 固定 revision counter 行不存在且数据库已经存在零个或多个已提交 policy revision
- **THEN** 当前在线 mutation transaction MUST 读取最大已提交 revision，并通过 Ent 幂等创建与该值对齐的固定 counter 行
- **AND** transaction MUST 在初始化后原子递增 counter，使新 revision 大于全部已有 revision
- **AND** migration MUST NOT 依赖手写 seed `INSERT` SQL 创建 counter 行

#### Scenario: 并发首次初始化保持提交顺序

- **WHEN** 多个在线 RBAC mutation 并发发现固定 counter 行不存在
- **THEN** 固定主键冲突和 counter 行锁 MUST 串行化初始化与后续递增
- **AND** 每个成功 mutation MUST 获得唯一连续 revision，较大 revision MUST NOT 先于较小 revision 提交

#### Scenario: 初始化或事实写入失败完整回滚

- **WHEN** counter 初始化、递增、revision、outbox 或 transaction commit 任一步失败
- **THEN** 当前 transaction 的业务 mutation、counter 变化、revision 和 outbox MUST 全部回滚
- **AND** 后续重试 MUST 能重新执行缺失初始化或正常原子递增
