## MODIFIED Requirements

### Requirement: RBAC watcher 自恢复生命周期与权威校准状态

RBAC watcher MUST 在单一显式业务生命周期内消费 `common/runtime/redispubsub` 提供的 Redis policy refresh 消息并持续执行 PostgreSQL policy revision 权威校准。通用 subscriber 的订阅故障 MUST NOT 终止数据库补偿；瞬时错误恢复后 watcher MUST 映射当前 subscription 状态且不得因历史错误保持永久失败。watcher MUST 只通过 permission application 拥有的结构化只读 status port 暴露组合状态，MUST NOT 保留 `Running()`/`LastError()` 旧接口、旧状态 adapter 或兼容分支。

#### Scenario: 启动订阅失败后自动恢复

- **WHEN** 通用 subscriber 初次创建订阅或等待订阅确认时发生瞬时错误
- **THEN** subscriber MUST 关闭本次 PubSub，并按带抖动且不超过配置最大值的指数退避持续创建新订阅
- **AND** 重试 MUST NOT 设置永久终止次数；成功确认订阅后 watcher 映射的 subscription state MUST 为 `connected`，最后订阅成功时间、当前 subscription 错误和重连次数 MUST 来自 subscriber 结构化状态
- **AND** watcher 的 RBAC 根生命周期 MUST 保持 running，MUST NOT 要求人工操作、进程重启或新的 RBAC mutation 才能恢复

#### Scenario: 运行期订阅终止后重建

- **WHEN** 已确认的 Pub/Sub 订阅在接收期间返回非取消错误、协议类型错误、连接终止或等价的不可继续接收状态
- **THEN** 通用 subscriber MUST 只关闭当前 PubSub 一次，将 subscription state 置为 `reconnecting` 并启动有界退避重订阅
- **AND** watcher MUST NOT 退出 RBAC 根生命周期或停止后续 PostgreSQL revision 周期补偿
- **AND** 重建成功后收到的重复、乱序或旧 hint MUST 继续遵守既有幂等和 projection revision 不倒退语义

#### Scenario: 权威校准独立于订阅状态

- **WHEN** watcher 启动、达到配置检查周期，或通用 subscriber 正在失败、背压和退避
- **THEN** watcher MUST 立即或按期直接读取 PostgreSQL latest policy revision，并在需要时执行 revision-aware reload
- **AND** 只有数据库查询成功且最终 projection ready、applied revision 不低于本次数据库目标时，watcher 才 MUST 更新最后权威校准成功时间并清除当前 reconcile 错误
- **AND** 数据库查询成功但 reload 失败、被取消或未达到目标时 MUST NOT 刷新成功时间或宣称校准成功

#### Scenario: 订阅与校准错误分别恢复

- **WHEN** subscription 或 reconcile 路径发生错误
- **THEN** watcher 结构化 status MUST 分别记录两条路径的固定低基数当前错误类别，将 subscriber 的 `none`、`subscribe_failed`、`receive_failed` 和 `protocol_failed` 显式映射到 RBAC application 状态，并将底层 cause 仅保留在日志中
- **AND** watcher 报告的 `LastFailureAt` MUST 取 subscriber 最近失败时间与 watcher 自行维护的 reconcile 最近失败时间中的较新值
- **WHEN** 同一路径随后成功确认订阅或完成权威校准
- **THEN** watcher MUST 清除该路径的当前错误类别并保留可诊断的历史时间，MUST NOT 清除另一条仍未恢复路径的当前错误

#### Scenario: 配置边界

- **WHEN** user-service 加载 `rbac.policy_watcher` 配置
- **THEN** 系统 MUST 支持正数 `check_interval`、`subscribe_timeout`、`max_staleness`、`retry_backoff.initial` 和 `retry_backoff.max`
- **AND** `retry_backoff.max` MUST 大于或等于 `retry_backoff.initial`，`max_staleness` MUST 大于 `check_interval`，非法配置 MUST 在应用启动前被拒绝
- **AND** permission composition MUST 将 `subscribe_timeout` 与两个 backoff 值显式传入 `redispubsub.Options`，并显式传入 `BufferSize: 64`；`WatcherSettings` MUST 只保留 `CheckInterval`
- **AND** `common/runtime/redispubsub` MUST NOT 提供业务默认值，系统 MUST NOT 读取旧 watcher 配置名、别名或回退配置分支

#### Scenario: 停止时无 goroutine 和订阅泄漏

- **WHEN** Fx lifecycle 调用 watcher `Stop` 或启动回滚取消 watcher
- **THEN** watcher root context 与 subscriber root context MUST 共同取消订阅确认、Receive、退避 timer、消息缓冲交付和周期校准，并等待各自唯一 drain 完成
- **AND** 当前 PubSub MUST 只关闭一次，共享 Redis client MUST NOT 被 watcher 或 subscriber 关闭，取消后 MUST NOT 再创建订阅
- **AND** Stop 超时 MUST 只结束当前调用方等待，后台 drain MUST 继续；后续 Stop MUST 等待同一个完成状态，`Messages()` MUST 只在 subscriber 完全停止后关闭
- **AND** 正常停止 MUST 将结构化状态置为 stopped 且不得记录为非预期后台错误

#### Scenario: applied revision、lag 与健康语义

- **WHEN** 系统报告本地applied revision、policy reload status或reload lag
- **THEN** local applied值 MUST 来自engine当前实际授权投影，lag MUST 计算为`max(known_latest_database_revision - engine_applied_revision, 0)`
- **AND** reload失败、消息接收、subscriber状态或Redis revision更新 MUST NOT 提升applied revision或将lag错误清零
- **WHEN** lag为0且latest revision已知
- **THEN** engine 实际投影 revision MUST 大于或等于该 latest revision，且最近 reload 状态 MUST 成功，系统才可仅基于 policy projection 判定 readiness/startup 健康
- **WHEN** engine未初始化、最近reload失败或applied revision低于已知target
- **THEN** readiness/startup MUST 报告policy不可用并拒绝业务流量

### Requirement: RBAC 架构装配与资源生命周期

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。permission application MUST 只保留权限查询、授权、policy loading/sync 和 seed/角色绑定所需最小端口，不得保留公开权限 command 或 route diff 生产装配。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL、HTTP response 和 named resource metadata MUST 留在对应边界。通用 Redis subscription lifecycle MUST 由 `common/runtime/redispubsub` 拥有，RBAC watcher MUST 只拥有消息消费、权威校准和业务状态组合；两者以及 policy 投影主动资源 MUST 显式启动、停止和回滚。无后台执行的 user-role localcache MUST NOT 拥有启停或关闭生命周期。permission composition MUST 以单一 runtime 聚合对象表达稳定组件集合。

#### Scenario: 分层、bootstrap 与最小依赖

- **WHEN** role 或 permission application service 被构造
- **THEN** 调用方 MUST 能以普通强类型参数提供 store、lookup、notifier 和 logger
- **AND** application/domain MUST NOT import Fx、嵌入 `fx.In` 或声明 DI tag
- **AND** 消费侧 application MUST 定义最小 port，feature MUST NOT 导入其他 feature 的 infrastructure 或 HTTP transport，role 仍使用的 permission lookup MUST 保留
- **WHEN** 实现超级管理员 bootstrap
- **THEN** application service MUST 位于 `user-service/internal/features/role/application/bootstrap/`，通过最小 `BootstrapStore` 调用 role infrastructure 中的 PostgreSQL adapter
- **AND** application MUST 负责输入归一化、密码策略、哈希及固定用户/角色 ID，MUST NOT 导入 Ent predicate、HTTP、Gin、Fx、SQL、Redis 或 datastore concrete implementation

#### Scenario: adapter、composition 与运行时聚合

- **WHEN** 构造 RBAC store、loader、engine、watcher、subscriber、cache 或 adapter
- **THEN** constructor MUST 接收普通强类型参数或无 DI metadata 的 options，MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag、named/group result
- **AND** named `primary_db`、`cache_redis`、optional、group 和 lifecycle 选择 MUST 留在 feature composition
- **AND** public provider MUST 只暴露 controller、authorizer、health/status、runtime 聚合对象和 application port，父 module MUST NOT 消费 infrastructure concrete implementation
- **WHEN** composition 提供 RBAC runtime 组件
- **THEN** composition MUST 使用共享 Redis client 与 `Store.PolicyChannel()` 构造一个具体 `*redispubsub.Subscriber`，再通过生产 watcher constructor 注入该实例
- **AND** composition MUST 通过单一 permission runtime 聚合对象表达已经构造的稳定接口或私有 lifecycle contract，MUST NOT 重建 engine、store、subscriber、watcher、version tracker、cache、resolver、Redis client 或 Ent client，application/domain MUST NOT 依赖该对象

#### Scenario: 有状态资源单实例与必需依赖

- **WHEN** composition 暴露同一有状态组件的多个接口视图
- **THEN** 系统 MUST 只构造一个实例并以普通 Go 赋值暴露，MUST NOT 重复构造 engine、store、subscriber、version tracker、watcher 或 cache
- **AND** watcher 的状态和运行器视图 MUST 指向同一 watcher，watcher 内部 MUST 通过 feature-local `messageSource` 最小接口消费同一 subscriber
- **WHEN** 角色、角色权限或用户角色写侧服务装配
- **THEN** 服务 MUST 具备可用 notifier；缺少 notifier 或安全 collaborator 时 constructor MUST 返回明确 error并拒绝装配，MUST NOT panic
- **AND** 系统 MUST NOT 用 no-op、nil fallback、deprecated alias 或兼容 wrapper 跳过 reload、Redis version、subscriber 或 watcher 同步

#### Scenario: watcher 与 cache lifecycle

- **WHEN** user-service 启停 permission/RBAC runtime
- **THEN** `NewSubscriber` 与 `NewWatcher` MUST 只构造对象，MUST NOT 启动 goroutine、订阅 Redis 或执行补偿循环
- **AND** hook MUST 初始加载 policy 后调用返回 `error` 的 watcher `Start()`；重复 Start MUST 幂等，停止已经开始后 Start MUST 返回 stopped error
- **AND** `Stop(ctx)` MUST 取消内部 context 并在调用方期限内等待 watcher 与 subscriber 退出；Stop 超时 MUST 返回 context 错误并保持后台 drain 与重复停止安全
- **AND** watcher 或后续 dispatcher 启动失败时，Fx lifecycle MUST 停止已经启动的 watcher，MUST NOT 忽略 `Start()` 错误或保留旧的无返回值 runner 签名
- **WHEN** user-role localcache 被构造或应用停止
- **THEN** cache MUST 作为无后台 goroutine 的普通对象使用，Fx result 与 hook MUST NOT 为其提供或调用 `Start(context.Context) error`、`Close() error`、closed state 或 lifecycle rollback

#### Scenario: 共享资源所有权与关闭安全

- **WHEN** RBAC 关闭 watcher、subscriber、store 或其他主动资源
- **THEN** `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源，subscriber MUST 只关闭自己当前持有的 PubSub attempt
- **AND** 关闭后授权 MUST 继续 fail-closed，不得因本地资源不可用产生允许结果
- **AND** RBAC MUST NOT 把服务业务配置、权限基线、角色值、消息 envelope、revision、outbox、缓存失效或 key schema 下沉到 `common`
