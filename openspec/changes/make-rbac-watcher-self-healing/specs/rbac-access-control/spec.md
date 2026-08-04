## ADDED Requirements

### Requirement: RBAC watcher 自恢复生命周期与权威校准状态

RBAC watcher MUST 在单一显式生命周期内持续监督 Redis policy refresh 订阅与 PostgreSQL policy revision 权威校准。订阅故障 MUST NOT 终止数据库补偿；瞬时错误恢复后 MUST 更新当前状态且不得因历史错误保持永久失败。watcher MUST 只通过 permission application 拥有的结构化只读 status port 暴露状态，MUST NOT 保留 `Running()`/`LastError()` 旧接口、旧状态 adapter 或兼容分支。

#### Scenario: 启动订阅失败后自动恢复

- **WHEN** watcher 初次创建订阅或等待订阅确认时发生瞬时错误
- **THEN** watcher MUST 关闭本次 PubSub，并按带抖动且不超过配置最大值的指数退避持续创建新订阅
- **AND** 重试 MUST 不设置永久终止次数，成功确认订阅后 MUST 将 subscription state 置为 `connected`、记录最后订阅成功时间、清除当前 subscription 错误并重置退避
- **AND** watcher 根生命周期 MUST 保持 running，MUST NOT 要求人工操作、进程重启或新的 RBAC mutation 才能恢复

#### Scenario: 运行期订阅终止后重建

- **WHEN** 已确认的 Pub/Sub 订阅在接收期间返回非取消错误、连接终止或等价的不可继续接收状态
- **THEN** watcher MUST 只关闭当前 PubSub 一次，将 subscription state 置为 `reconnecting` 并启动有界退避重订阅
- **AND** watcher MUST NOT 退出根生命周期或停止后续 PostgreSQL revision 周期补偿
- **AND** 重建成功后收到的重复、乱序或旧 hint MUST 继续遵守既有幂等和 projection revision 不倒退语义

#### Scenario: 权威校准独立于订阅状态

- **WHEN** watcher 启动、达到配置检查周期，或订阅正在失败和退避
- **THEN** watcher MUST 立即或按期直接读取 PostgreSQL latest policy revision，并在需要时执行 revision-aware reload
- **AND** 只有数据库查询成功且最终 projection ready、applied revision 不低于本次数据库目标时，才 MUST 更新最后权威校准成功时间并清除当前 reconcile 错误
- **AND** 数据库查询成功但 reload 失败、被取消或未达到目标时 MUST NOT 刷新成功时间或宣称校准成功

#### Scenario: 订阅与校准错误分别恢复

- **WHEN** subscription 或 reconcile 路径发生错误
- **THEN** 结构化 status MUST 记录对应路径的固定低基数当前错误类别和最近失败时间，并将底层 cause 仅保留在日志中
- **WHEN** 同一路径随后成功确认订阅或完成权威校准
- **THEN** watcher MUST 清除该路径的当前错误类别并保留可诊断的历史时间，MUST NOT 清除另一条仍未恢复路径的当前错误

#### Scenario: 配置边界

- **WHEN** user-service 加载 `rbac.policy_watcher` 配置
- **THEN** 系统 MUST 支持正数 `check_interval`、`subscribe_timeout`、`max_staleness`、`retry_backoff.initial` 和 `retry_backoff.max`
- **AND** `retry_backoff.max` MUST 不小于 `retry_backoff.initial`，`max_staleness` MUST 大于 `check_interval`，非法配置 MUST 在应用启动前被拒绝
- **AND** 系统 MUST NOT 读取旧 watcher 配置名、别名或回退配置分支

#### Scenario: 停止时无 goroutine 和订阅泄漏

- **WHEN** Fx lifecycle 调用 watcher `Stop` 或启动回滚取消 watcher
- **THEN** 根 context MUST 取消订阅确认、Receive、退避 timer、payload 交付和周期校准，并等待全部 watcher goroutine 退出
- **AND** 当前 PubSub MUST 只关闭一次，共享 Redis client MUST NOT 被 watcher 关闭，取消后 MUST NOT 再创建订阅
- **AND** 正常停止 MUST 将结构化状态置为 stopped 且不得记录为非预期后台错误
