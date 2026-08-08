## MODIFIED Requirements

### Requirement: Casbin 授权、策略缓存与多副本同步

系统 MUST 在认证后使用 RBAC 中间件保护权限、角色和用户业务接口，并以 PostgreSQL 关系数据作为业务权威来源。系统 MUST 将会改变 Casbin 静态授权规则集合的事实记录为单调 policy revision，并将用户角色绑定变更记录为独立 user-role revision 或等价提交水位；两类 revision MUST NOT 混用。每个本地 Casbin enforcer MUST 与其实际加载的数据库 policy revision 绑定，applied revision MUST 表示该 engine 当前成功应用的授权投影，MUST NOT 表示 Redis 通知序号、用户角色绑定提交水位、消息接收进度或 reload attempt。授权 MUST 使用稳定 subject、Gin route template 和 HTTP method，并在任何身份、策略、revision 或执行异常下 fail-closed。Redis 与 Pub/Sub MUST 只传播数据库提交事实并加速副本收敛；授权热路径 MUST 使用本地投影，MUST NOT 每请求读取 Redis 或 PostgreSQL revision。

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
- **WHEN** 用户角色回源失败、context 取消、连续失效、Casbin 执行错误、policy 未加载、目标 policy revision 未追平或最近一次 policy reload 失败
- **THEN** 系统 MUST 拒绝请求并暴露 policy 不可用 readiness/startup 状态，MUST NOT 使用旧角色集合或保留的旧 enforcer 继续允许请求
- **WHEN** 请求命中显式授权白名单或使用 `OPTIONS`
- **THEN** 中间件 MUST 允许请求并 MUST NOT 调用授权服务
- **WHEN** 注册 `/api/v1` 权限、角色和用户业务路由
- **THEN** 这些路由 MUST 经过当前认证和 RBAC 中间件链；token version validator、RBAC authorizer 或必需 route registrar 缺失时系统 MUST 拒绝注册部分路由

#### Scenario: revision-aware policy 快照加载

- **WHEN** policy loader 面向目标数据库 policy revision 构造授权策略
- **THEN** loader MUST 在同一 PostgreSQL 一致性快照中读取可见 latest policy revision 与启用角色、角色权限绑定和 permissions 投影，并返回 `PolicySet{Revision, PermissionRules}` 或等价结构
- **AND** 返回的 revision MUST 大于或等于目标 policy revision，规则 MUST 与该 revision 所属数据库快照绑定；loader MUST NOT 为旧规则附加较新的 revision
- **AND** 用户身份解析 MUST 排除已软删除用户，loader MUST NOT 使用权限 active predicate，独立 `casbin_rules` 表 MUST NOT 成为业务权威来源
- **WHEN** 当前快照可见 policy revision 低于目标 policy revision
- **THEN** loader MUST 在 context 期限内结束旧快照并使用新快照重试，MUST NOT 返回低于目标的 policy、在旧快照内无限等待或将通知 revision 直接作为快照 revision
- **WHEN** target policy revision 为 0 且数据库尚无 policy revision 记录
- **THEN** loader MUST 以 revision 0 加载当前基线投影，并保持超级管理员 wildcard policy 语义
- **WHEN** 只有用户角色绑定发生变化且未改变角色状态、角色权限绑定或 permissions 投影
- **THEN** 系统 MUST NOT 调用 policy loader，MUST NOT 查询 `role_permissions` 全集，MUST NOT 构造新的 Casbin enforcer

#### Scenario: revision-aware engine 交换与防倒退

- **WHEN** engine 收到目标 policy revision 并完成候选 `PolicySet` 与 enforcer 构造
- **THEN** engine MUST 在同一锁定临界区比较候选 revision 与当前 applied revision，并原子交换 enforcer、applied revision 与成功状态
- **AND** 只有更高候选 revision 可以替换当前 enforcer，相等候选 MUST 幂等成功，较低候选 MUST 被丢弃且不得覆盖或降低当前投影
- **WHEN** policy revision 1 的 reload 在 policy revision 2 成功应用后才完成
- **THEN** 最终 enforcer 和 applied revision MUST 仍对应 policy revision 2 或更高的数据库快照
- **AND** engine、tracker/status、metrics 与 health 暴露的 applied revision MUST 来自同一实际投影状态，MUST NOT 由 watcher 独立推进
- **WHEN** user-role revision 高于当前 Casbin applied policy revision
- **THEN** engine applied revision MUST 保持不变，MUST NOT 把 user-role revision 当成 policy target 或 applied revision

#### Scenario: 同实例并发 reload 收敛

- **WHEN** 同一实例并发收到多个数据库 policy target revision
- **THEN** engine MUST 串行化或 coalesce 实际 reload 工作，将 pending target 单调提升到已观察到的最大 policy revision，并防止并发构造导致投影倒退
- **AND** 等待方只有在实际 applied revision 不低于其 target policy revision 时才能观察到成功；单个等待方 context 取消 MUST NOT 取消其他调用仍需要的共享 reload
- **WHEN** 100 个并发 policy 写入触发 reload 且数据库 latest policy revision 可见
- **THEN** reload 稳定后 engine applied revision MUST 等于加载时数据库 latest policy revision 且不低于全部 policy target 中的最大值
- **AND** 系统 MUST NOT 要求 policy revision 连续或为每个中间 policy revision 分别构造 enforcer

#### Scenario: 初始加载、reload 失败与恢复

- **WHEN** user-service 启动 permission/RBAC 模块
- **THEN** composition 层 MUST 使用可取消或带超时的启动 context 显式加载当前数据库 latest policy revision
- **WHEN** 初始加载失败、被取消或不能达到目标 policy revision
- **THEN** engine MUST 保留实际 applied revision、记录最近错误和 reload 失败指标，后续授权 MUST fail-closed，`app.Start` MUST 保持成功
- **AND** reload 状态和 readiness/startup MUST 保留失败信息并拒绝接入业务流量
- **WHEN** 已存在成功投影后的 reload 加载、构造或交换失败
- **THEN** engine MUST 保留上一成功 enforcer 及其 applied revision，MUST NOT 提升 revision、清除失败或使用旧投影放行请求
- **WHEN** 后续显式 reload、Pub/Sub 或周期补偿成功应用不低于目标的数据库 policy 快照
- **THEN** engine MUST 原子替换或确认当前投影、清除最近 reload 错误并恢复 readiness/startup

#### Scenario: 用户角色缓存键、容量与值隔离

- **WHEN** user-role cache 启用
- **THEN** permission feature MUST 在 localcache 边界使用 `userID.String()` 作为规范 string key，并将配置的正数 size 映射为最大 item 数
- **AND** common MUST NOT 接收 key encoder、解析业务 UUID 或暴露底层 cache option
- **WHEN** 缓存命中
- **THEN** loader 写入缓存前和 `RolesForUser` 返回前 MUST 复制 `[]uuid.UUID`，调用方修改返回 slice MUST NOT 污染缓存或后续读取
- **AND** `common/runtime/localcache` MUST NOT 承担角色 ID clone 语义

#### Scenario: 用户角色回源与缓存失效

- **WHEN** 缓存未命中
- **THEN** 系统 MUST 合并同一用户的并发回源并查询 PostgreSQL 中的当前启用角色，loader 错误 MUST NOT 写入缓存
- **WHEN** 回源失败或结果连续两次与失效并发
- **THEN** 授权 MUST fail-closed，MUST NOT 因 cache 不可用产生允许结果
- **WHEN** `rbac.user_role_cache.enabled=false`
- **THEN** 系统 MUST 直接回源、返回独立角色 ID slice 并保持 fail-closed；direct stats source MUST 使用 `LoadSuccess` 与 `LoadError` 表达逐次结果

#### Scenario: 在线写后同步与数据库 revision 目标

- **WHEN** 角色状态、角色权限或权限投影通过在线 API 或受控维护流程与 policy revision 原子提交成功
- **THEN** 本实例 coordinator MUST 使用该数据库 policy revision 作为 reload 目标，outbox dispatcher MUST 传播同一数据库 policy revision
- **AND** reload 或通知失败 MUST 保持可诊断和 fail-closed 语义，cache invalidation MUST 保持同步幂等，MUST NOT 把通知接收、Redis publish 成功或 user-role revision 标记为 engine 已应用
- **AND** `PolicyChangeNotifier` 或等价 policy reload port MUST 是正式 command service 的必需依赖并接收数据库 policy revision
- **WHEN** 用户角色绑定通过在线 API 与 user-role revision 原子提交成功
- **THEN** 本实例 coordinator MUST 只执行指定用户角色缓存失效，outbox dispatcher MUST 传播同一 user-role revision 和 user ID，MUST NOT 调用 Casbin policy reload 或 policy loader
- **AND** user-role cache invalidation 失败 MUST 保持可诊断和 fail-closed 语义，MUST NOT 推进 Casbin target revision 或 applied revision
- **WHEN** 权限投影由离线 migration、seed 或 bootstrap 改变
- **THEN** 离线命令 MUST NOT 宣称已完成在线 policy refresh，运维 MUST 显式创建/传播对应 policy revision、执行 revision-aware reload 或滚动重启副本

#### Scenario: watcher、重复通知与副本收敛

- **WHEN** watcher 通过 Pub/Sub 或周期性检查发现数据库 policy revision 高于 engine applied revision
- **THEN** watcher MUST 以该 policy revision 调用 revision-aware application port，只有 engine 成功应用不低于 target 的投影后才能将该 policy revision 视为 applied
- **AND** Pub/Sub 丢失时周期性 policy revision 补偿 MUST 使副本最终收敛
- **WHEN** watcher 收到重复、相等或乱序 policy 通知
- **THEN** policy reload MUST 保持幂等且不得倒退 enforcer；已应用且 projection ready 的 policy revision MUST 跳过全量 reload
- **AND** watcher MUST 合并当前待处理通知，对同批 policy 通知只针对最高未应用 policy revision 构造一次 Casbin enforcer
- **WHEN** watcher 收到 user-role 通知
- **THEN** watcher MUST 执行消息要求的用户角色 cache invalidation 副作用，MUST NOT 独立推进 Casbin engine applied revision 或伪造 policy reload 完成
- **AND** 定向 user-role invalidation 通知携带可用 user ID 时 MUST 失效该用户缓存；存在 revision gap、无法证明精确用户集合完整或缺少可用 user ID 时 MUST 失效全部用户角色缓存
- **WHEN** watcher 处理 100 条重复或连续通知且其中没有新的未应用 policy revision
- **THEN** policy loader 调用次数 MUST 有常数上界；纯 user-role 通知 MUST 产生 0 次 policy loader 调用
