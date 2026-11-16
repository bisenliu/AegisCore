## MODIFIED Requirements

### Requirement: Casbin 授权、策略缓存与多副本同步

系统 MUST 在认证后使用 RBAC 中间件保护权限、角色和用户业务接口，并以 PostgreSQL 关系数据作为业务权威来源。系统 MUST 将会改变 Casbin 静态授权规则集合的事实记录为单调 policy revision，并将用户角色绑定变更记录为独立 user-role revision；两类 revision MUST NOT 混用。每个本地 Casbin enforcer MUST 与其实际加载的数据库 policy revision 绑定。授权热路径 MUST 使用本地投影，MUST NOT 每请求读取 Redis 或 PostgreSQL revision。Redis Pub/Sub MUST 只作为可丢失的加速 hint。系统 MUST NOT 使用事务 outbox、claim lease、dispatcher 或可靠投递状态机同步 RBAC。

#### Scenario: 在线 policy 写入后的副本收敛

- **WHEN** 角色状态、角色权限或权限投影通过在线 API 与 policy revision 原子提交成功
- **THEN** 本实例 coordinator MUST 使用该数据库 policy revision 作为 reload 目标
- **AND** 系统 MUST best-effort 发布同一 policy revision 的 Pub/Sub hint
- **AND** Pub/Sub 失败或丢失时，watcher 的周期性数据库 policy revision 补偿 MUST 使副本最终收敛
- **AND** 系统 MUST NOT 持久化 outbox event 或启动 dispatcher 重试该 hint

#### Scenario: 在线 user-role 写入后的副本收敛

- **WHEN** 用户角色绑定通过在线 API 与 user-role revision 原子提交成功
- **THEN** 本实例 coordinator MUST 只失效指定用户角色缓存，MUST NOT 调用 Casbin policy reload
- **AND** 系统 MUST best-effort 发布包含同一 user-role revision 和 user ID 的 Pub/Sub hint
- **AND** hint 丢失时，watcher MUST 读取高于本实例已观察水位的 user-role revision，并失效对应 user ID 的缓存
- **AND** revision 查询失败或待补偿 revision 超过有界批次时，watcher MUST 清空本实例全部用户角色缓存

### Requirement: RBAC policy revision、同步与事务 outbox

系统 MUST 以 PostgreSQL 中追加式 policy revision 与 user-role revision 作为在线 RBAC 写入和跨副本恢复的权威事实。revision counter、业务 mutation 与 revision MUST 在同一 transaction 中保持提交顺序和原子性。系统 MUST NOT 在该 transaction 或其后台流程中创建、claim、ack 或重试 outbox event。API MUST 以业务 transaction 是否提交表达写入结果，提交后的本地同步或 Pub/Sub 发布失败不得把已提交 mutation 返回为失败。

#### Scenario: 在线 mutation 原子提交 revision

- **WHEN** 在线 RBAC mutation 成功提交
- **THEN** 数据库 MUST 同时存在对应业务变更和一条已提交 revision
- **AND** 数据库 MUST NOT 存在该 revision 对应的 outbox event

#### Scenario: 写入失败完整回滚

- **WHEN** counter 初始化、递增、revision 插入或 transaction commit 任一步失败
- **THEN** 当前 transaction 的业务 mutation、counter 变化和 revision MUST 全部回滚

#### Scenario: 提交后的本地同步失败

- **WHEN** RBAC mutation 与 revision 已提交，但本实例 reload、缓存失效或 Redis publish 失败
- **THEN** API MUST 返回该 mutation 的成功结果
- **AND** 本实例授权或缓存 MUST 保持 fail-closed，其他副本 MUST 通过数据库 revision 补偿最终收敛
