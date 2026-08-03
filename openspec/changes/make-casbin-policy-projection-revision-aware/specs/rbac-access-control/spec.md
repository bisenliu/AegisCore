## MODIFIED Requirements

### Requirement: Casbin 授权、策略缓存与多副本同步

系统 MUST 在认证后使用 RBAC 中间件保护权限、角色和用户业务接口，并以 PostgreSQL 关系数据与单调 policy revision 作为业务权威来源，以本地 Casbin policy 和用户角色 loading cache 作为授权投影。每个本地 Casbin enforcer MUST 与其实际加载的数据库 policy revision 绑定，applied revision MUST 表示该 engine 当前成功应用的授权投影，MUST NOT 表示 Redis 通知序号、消息接收进度或 reload attempt。授权 MUST 使用稳定 subject、Gin route template 和 HTTP method，并在任何身份、策略、revision 或执行异常下 fail-closed。Redis 与 Pub/Sub MUST 只传播数据库 revision并加速副本收敛；授权热路径 MUST 使用本地投影，MUST NOT 每请求读取 Redis或PostgreSQL revision。

#### Scenario: 授权请求与超级管理员

- **WHEN** 已认证请求进入受保护的 `/api/v1` 路由
- **THEN** 中间件 MUST 使用请求上下文中的用户 ID、Gin `FullPath()` 和 HTTP method 构造授权请求
- **AND** 用户与角色 subject MUST 分别使用 `user:<user_uuid>` 和 `role:<role_uuid>`
- **WHEN** 用户当前启用角色拥有匹配 method 和 route template 的权限
- **THEN** 系统 MUST 允许请求进入 controller，否则 MUST 返回禁止访问错误
- **WHEN** 用户拥有 `rbacbaseline` 定义的内置超级管理员角色
- **THEN** policy loader MUST 为该角色提供受保护业务接口的 wildcard policy，超级管理员角色常量 MUST 只由 `rbacbaseline` 提供

#### Scenario: fail-closed 与路由注册

- **WHEN** 请求缺少用户 ID、用户 ID 类型非法或 subject 不能解析为用户 UUID
- **THEN** 系统 MUST 返回未认证错误并拒绝请求，且 MUST NOT 调用 Casbin engine
- **WHEN** 用户角色回源失败、context 取消、Casbin 执行错误、policy 未加载、目标 revision 未追平或最近一次 reload 失败
- **THEN** 系统 MUST 拒绝请求并暴露 policy 不可用 readiness/startup 状态，MUST NOT 使用保留的旧 enforcer 继续允许请求
- **WHEN** 请求命中显式授权白名单或使用 `OPTIONS`
- **THEN** 中间件 MUST 允许请求并 MUST NOT 调用授权服务
- **WHEN** 注册 `/api/v1` 权限、角色和用户业务路由
- **THEN** 这些路由 MUST 经过当前认证和 RBAC 中间件链；token version validator、RBAC authorizer 或必需 route registrar 缺失时系统 MUST 拒绝注册部分路由

#### Scenario: revision-aware policy 快照加载

- **WHEN** policy loader 面向目标数据库 revision 构造授权策略
- **THEN** loader MUST 在同一 PostgreSQL 一致性快照中读取可见 latest policy revision 与启用角色、角色权限绑定和 permissions 投影，并返回 `PolicySet{Revision, PermissionRules}` 或等价结构
- **AND** 返回的 revision MUST 不低于目标 revision，规则 MUST 与该 revision 所属数据库快照绑定；loader MUST NOT 为旧规则附加较新的 revision
- **AND** 用户身份解析 MUST 排除已软删除用户，loader MUST NOT 使用权限 active predicate，独立 `casbin_rules` 表 MUST NOT 成为业务权威来源
- **WHEN** 当前快照可见 revision 低于目标 revision
- **THEN** loader MUST 在 context 期限内结束旧快照并使用新快照重试，MUST NOT 返回低于目标的 policy、在旧快照内无限等待或将通知 revision直接作为快照 revision
- **WHEN** target revision 为 0且数据库尚无 policy revision记录
- **THEN** loader MUST 以revision 0加载当前基线投影，并保持超级管理员wildcard policy语义

#### Scenario: revision-aware engine 交换与防倒退

- **WHEN** engine 收到目标 revision 并完成候选 `PolicySet` 与 enforcer 构造
- **THEN** engine MUST 在同一锁定临界区比较候选 revision与当前 applied revision，并原子交换 enforcer、applied revision与成功状态
- **AND** 只有更高候选 revision可以替换当前enforcer，相等候选 MUST 幂等成功，较低候选 MUST 被丢弃且不得覆盖或降低当前投影
- **WHEN** revision 1的reload在revision 2成功应用后才完成
- **THEN** 最终enforcer和applied revision MUST 仍对应revision 2或更高的数据库快照
- **AND** engine、tracker/status、metrics与health暴露的applied revision MUST 来自同一实际投影状态，MUST NOT由watcher独立推进

#### Scenario: 同实例并发 reload 收敛

- **WHEN** 同一实例并发收到多个数据库target revision
- **THEN** engine MUST 串行化或coalesce实际reload工作，将pending target单调提升到已观察到的最大值，并防止并发构造导致投影倒退
- **AND** 等待方只有在实际applied revision不低于其target时才能观察到成功；单个等待方context取消 MUST NOT取消其他调用仍需要的共享reload
- **WHEN** 100个并发policy写入触发reload且数据库latest revision可见
- **THEN** reload稳定后engine applied revision MUST 等于加载时数据库latest revision且不低于全部target中的最大值
- **AND** 系统 MUST NOT要求revision连续或为每个中间revision分别构造enforcer

#### Scenario: 初始加载、reload 失败与恢复

- **WHEN** user-service启动permission/RBAC模块
- **THEN** composition层 MUST使用可取消或带超时的启动context显式加载当前数据库latest policy revision
- **WHEN** 初始加载失败、被取消或不能达到目标revision
- **THEN** engine MUST保留实际applied revision、记录最近错误和reload失败指标，后续授权 MUST fail-closed，`app.Start` MUST保持成功
- **AND** reload状态和readiness/startup MUST保留失败信息并拒绝接入业务流量
- **WHEN** 已存在成功投影后的reload加载、构造或交换失败
- **THEN** engine MUST保留上一成功enforcer及其applied revision，MUST NOT提升revision、清除失败或使用旧投影放行请求
- **WHEN** 后续显式reload、Pub/Sub或周期补偿成功应用不低于目标的数据库快照
- **THEN** engine MUST原子替换或确认当前投影、清除最近reload错误并恢复readiness/startup

#### Scenario: 用户角色缓存键、容量与值隔离

- **WHEN** user-role cache 启用
- **THEN** permission feature MUST 使用 `uuid.UUID` 作为真实业务 key，并将配置的正数 size 映射为最大 item 数
- **AND** common MUST NOT 字符串化 UUID、接收 key encoder 或暴露底层 cache option
- **WHEN** 缓存命中
- **THEN** loader 写入缓存前和 `RolesForUser` 返回前 MUST 复制 `[]uuid.UUID`，调用方修改返回 slice MUST NOT 污染缓存或后续读取
- **AND** `common/runtime/localcache` MUST NOT 承担角色 ID clone 语义

#### Scenario: 用户角色回源与缓存关闭

- **WHEN** 缓存未命中
- **THEN** 系统 MUST 合并同一用户的并发回源并查询 PostgreSQL 中的当前启用角色，loader 错误 MUST NOT 写入缓存
- **WHEN** cache 已关闭或回源失败
- **THEN** 授权 MUST fail-closed，MUST NOT 因 cache 不可用产生允许结果
- **WHEN** `rbac.user_role_cache.enabled=false`
- **THEN** 系统 MUST 直接回源、返回独立角色 ID slice并保持fail-closed；direct stats source MUST使用`LoadSuccess`与`LoadError`表达逐次结果

#### Scenario: 在线写后同步与数据库 revision 目标

- **WHEN** 角色状态、角色权限或用户角色绑定通过在线API与policy revision原子提交成功
- **THEN** 本实例coordinator MUST使用该数据库revision作为reload或cache invalidation目标，outbox dispatcher MUST传播同一数据库revision
- **AND** reload、cache invalidation或通知失败 MUST保持可诊断和fail-closed语义，MUST NOT把通知接收、Redis max写入或publish成功标记为engine已应用
- **AND** `PolicyChangeNotifier` MUST是正式command service的必需依赖并接收数据库revision
- **WHEN** 权限投影由离线migration、seed或bootstrap改变
- **THEN** 离线命令 MUST NOT宣称已完成在线policy refresh，运维 MUST显式创建/传播对应revision、执行revision-aware reload或滚动重启副本

#### Scenario: watcher、重复通知与副本收敛

- **WHEN** watcher通过Pub/Sub或周期性检查发现数据库policy revision高于engine applied revision
- **THEN** watcher MUST以该revision调用revision-aware application port，只有engine成功应用不低于target的投影后才能将该revision视为applied
- **AND** Pub/Sub丢失时周期性revision补偿 MUST使副本最终收敛
- **WHEN** watcher收到重复、相等或乱序通知
- **THEN** policy reload MUST保持幂等且不得倒退enforcer；消息kind要求的用户角色cache invalidation副作用 MUST仍按既有协议执行
- **AND** 定向user-role invalidation通知 MUST NOT独立推进Casbin engine applied revision或伪造policy reload完成

#### Scenario: applied revision、lag 与健康语义

- **WHEN** 系统报告本地applied revision、policy reload status或reload lag
- **THEN** local applied值 MUST来自engine当前实际授权投影，lag MUST计算为`max(known_latest_database_revision - engine_applied_revision, 0)`
- **AND** reload失败、消息接收或Redis revision更新 MUST NOT提升applied revision或将lag错误清零
- **WHEN** lag为0且latest revision已知
- **THEN** engine实际投影revision MUST不低于该latest revision，且最近reload状态 MUST成功，系统才可仅基于policy projection判定readiness/startup健康
- **WHEN** engine未初始化、最近reload失败或applied revision低于已知target
- **THEN** readiness/startup MUST报告policy不可用并拒绝业务流量
