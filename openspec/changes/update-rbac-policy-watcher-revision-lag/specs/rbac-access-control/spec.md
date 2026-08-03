## ADDED Requirements

### Requirement: RBAC watcher 以数据库 revision 补偿收敛

RBAC watcher MUST 以 PostgreSQL latest policy revision作为副本补偿和reload目标的权威来源，并以本地Casbin engine实际应用的projection revision作为本地状态。Redis Pub/Sub及其payload revision MUST只作为可丢失、可重复、可乱序的唤醒hint，Redis counter、key缺失或重建状态 MUST NOT决定副本已经收敛。授权热路径 MUST NOT因本要求增加PostgreSQL或Redis revision读取。

#### Scenario: Pub/Sub消息触发数据库revision校准

- **WHEN** watcher收到合法policy refresh消息
- **THEN** watcher MUST读取数据库latest policy revision并以该revision作为`ReloadToRevision`或等价revision-aware reload的目标
- **AND** payload revision MUST只作为hint和低风险诊断字段，MUST NOT直接推进local applied projection revision、清零lag或覆盖数据库latest revision
- **AND** payload重复、乱序或低于local applied revision时，engine投影 MUST保持不倒退，消息要求的既有cache side effect仍 MUST保持幂等语义

#### Scenario: Pub/Sub丢失后的周期补偿

- **WHEN** 数据库latest policy revision高于local applied projection revision且对应Pub/Sub消息丢失
- **THEN** 周期性`CheckVersion`或等价补偿检查 MUST直接读取数据库latest revision并触发revision-aware reload
- **AND** watcher MUST在后续成功检查与reload中最终使local applied projection revision不低于数据库latest revision
- **AND** 补偿判断 MUST NOT依赖Redis counter存在、领先或与数据库latest相等

#### Scenario: Redis状态不影响数据库补偿

- **WHEN** Redis counter不存在、落后于数据库latest、被重建为较小值或Redis从故障中恢复
- **THEN** watcher MUST继续以数据库latest revision判断是否需要reload
- **AND** 系统 MUST NOT因Redis值等于或低于local applied revision而跳过数据库revision超前所需的补偿
- **AND** Redis恢复后收到的旧消息 MUST NOT使旧revision覆盖新projection或降低local applied revision

#### Scenario: 数据库revision source不可用

- **WHEN** Pub/Sub唤醒或周期检查无法读取数据库latest policy revision
- **THEN** watcher MUST记录稳定的revision store unavailable诊断并保留底层cause用于日志
- **AND** watcher MUST NOT使用Redis counter或payload revision冒充数据库目标、记录reload success或把lag重置为`0`
- **AND** 后续数据库读取恢复时，周期检查或下一条hint MUST重新校准latest revision并继续补偿

#### Scenario: reload失败后恢复

- **WHEN** 数据库latest revision高于local applied revision但本地reload失败、被取消或未达到目标
- **THEN** engine MUST保留上一成功projection及其applied revision并保持fail-closed健康语义，watcher MUST记录reload failure且不得宣称收敛
- **AND** 后续Pub/Sub hint或周期检查 MUST再次读取数据库latest revision并重试
- **WHEN** 后续reload成功且实际applied revision不低于读取到的database latest revision
- **THEN** watcher MUST记录reload success并恢复收敛状态

#### Scenario: revision查询依赖边界

- **WHEN** permission feature查询latest policy revision
- **THEN** application MUST拥有只读最小revision source port，PostgreSQL/Ent adapter MUST留在permission infrastructure，named database与lifecycle选择 MUST留在composition
- **AND** revision查询语义 MUST NOT下沉到`common/`、`internal/shared/`或`internal/integration/`，application/domain MUST NOT导入Ent concrete client或predicate包
- **AND** 系统 MUST复用现有policy revision schema，MUST NOT为watcher新增revision、outbox schema或dispatcher
