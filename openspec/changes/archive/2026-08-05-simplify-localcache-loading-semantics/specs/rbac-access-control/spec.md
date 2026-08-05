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
- **WHEN** 用户角色回源失败、context 取消、连续失效、Casbin 执行错误、policy 未加载、目标 revision 未追平或最近一次 reload 失败
- **THEN** 系统 MUST 拒绝请求并暴露 policy 不可用 readiness/startup 状态，MUST NOT 使用旧角色集合或保留的旧 enforcer 继续允许请求
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
- **THEN** 系统 MUST 直接回源、返回独立角色 ID slice并保持fail-closed；direct stats source MUST使用`LoadSuccess`与`LoadError`表达逐次结果

#### Scenario: 在线写后同步与数据库 revision 目标

- **WHEN** 角色状态、角色权限或用户角色绑定通过在线API与policy revision原子提交成功
- **THEN** 本实例coordinator MUST使用该数据库revision作为reload或cache invalidation目标，outbox dispatcher MUST传播同一数据库revision
- **AND** reload或通知失败 MUST保持可诊断和fail-closed语义，cache invalidation MUST 保持同步幂等，MUST NOT把通知接收、Redis max写入或publish成功标记为engine已应用
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

### Requirement: RBAC 架构装配与资源生命周期

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。permission application MUST 只保留权限查询、授权、policy loading/sync 和 seed/角色绑定所需最小端口，不得保留公开权限 command 或 route diff 生产装配。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL、HTTP response 和 named resource metadata MUST 留在对应边界。RBAC watcher 和 policy 投影主动资源 MUST 显式启动、停止和回滚；无后台执行的 user-role localcache MUST 不拥有启停或关闭生命周期。permission composition MUST 以单一 runtime 聚合对象表达稳定组件集合。

#### Scenario: 分层、bootstrap 与最小依赖

- **WHEN** role 或 permission application service 被构造
- **THEN** 调用方 MUST 能以普通强类型参数提供 store、lookup、notifier 和 logger
- **AND** application/domain MUST NOT import Fx、嵌入 `fx.In` 或声明 DI tag
- **AND** 消费侧 application MUST 定义最小 port，feature MUST NOT 导入其他 feature 的 infrastructure 或 HTTP transport，role 仍使用的 permission lookup MUST 保留
- **WHEN** 实现超级管理员 bootstrap
- **THEN** application service MUST 位于 `user-service/internal/features/role/application/bootstrap/`，通过最小 `BootstrapStore` 调用 role infrastructure 中的 PostgreSQL adapter
- **AND** application MUST 负责输入归一化、密码策略、哈希及固定用户/角色 ID，MUST NOT 导入 Ent predicate、HTTP、Gin、Fx、SQL、Redis 或 datastore concrete implementation

#### Scenario: adapter、composition 与运行时聚合

- **WHEN** 构造 RBAC store、loader、engine、watcher、cache 或 adapter
- **THEN** constructor MUST 接收普通强类型参数或无 DI metadata 的 options，MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag、named/group result
- **AND** named `primary_db`、`cache_redis`、optional、group 和 lifecycle 选择 MUST 留在 feature composition
- **AND** public provider MUST 只暴露 controller、authorizer、health/status、runtime 聚合对象和 application port，父 module MUST NOT 消费 infrastructure concrete implementation
- **WHEN** composition 提供 RBAC runtime 组件
- **THEN** composition MUST 通过单一 permission runtime 聚合对象表达已经构造的稳定接口或私有 lifecycle contract
- **AND** 聚合对象 MUST NOT 重建 engine、store、watcher、version tracker、cache、resolver、Redis client 或 Ent client，application/domain MUST NOT 依赖该对象

#### Scenario: 有状态资源单实例与必需依赖

- **WHEN** composition 暴露同一有状态组件的多个接口视图
- **THEN** 系统 MUST 只构造一个实例并以普通 Go 赋值暴露，MUST NOT 重复构造 engine、store、version tracker、watcher 或 cache
- **AND** watcher 的状态和运行器视图 MUST 指向同一实例
- **WHEN** 角色、角色权限或用户角色写侧服务装配
- **THEN** 服务 MUST 具备可用 notifier；缺少 notifier 或安全 collaborator 时 constructor MUST 返回明确 error 并拒绝装配，MUST NOT panic
- **AND** 系统 MUST NOT 用 no-op、nil fallback 或兼容 wrapper 跳过 reload、Redis version 或 watcher 同步

#### Scenario: watcher 与 cache lifecycle

- **WHEN** user-service 启停 permission/RBAC runtime
- **THEN** `NewWatcher` MUST 只构造对象，MUST NOT 启动 goroutine、订阅 Redis 或执行补偿循环
- **AND** hook MUST 初始加载 policy 后启动 watcher；`Start()` 和 `Stop(ctx)` MUST 幂等，`Stop(ctx)` MUST 取消内部 context 并在调用方期限内等待 watcher 退出
- **AND** Stop 超时 MUST 返回 context 错误并保持重复停止安全，启动失败或停止时已启动 watcher MUST 被停止
- **WHEN** user-role localcache 被构造或应用停止
- **THEN** cache MUST 作为无后台 goroutine 的普通对象使用，Fx result 与 hook MUST NOT 为其提供或调用 `Start(context.Context) error`、`Close() error`、closed state 或 lifecycle rollback

#### Scenario: 共享资源所有权与关闭安全

- **WHEN** RBAC 关闭 watcher、store 或其他主动资源
- **THEN** `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源
- **AND** 关闭后授权 MUST 继续 fail-closed，不得因本地资源不可用产生允许结果
- **AND** RBAC MUST NOT 把服务业务配置、权限基线、角色值复制或 key schema 下沉到 `common`

### Requirement: 用户角色缓存失效顺序门禁

系统 MUST 使用 `common/runtime/localcache` 的业务中立 cache-wide revision 与发布门禁保护 user-role cache。用户角色缓存失效 MUST 在同一发布临界区提升 revision 并删除指定或全部缓存项；任何在失效前开始但未发布的旧回源结果 MUST NOT 在失效后写入缓存或返回给授权 caller。permission feature MUST NOT 维护 `userRoleCacheGeneration`、generation token、stale generation error 或等价重复门禁。cache disabled 模式 MUST 继续直接回源并保持 fail-closed。

#### Scenario: 单用户失效抑制旧 load 返回与写回

- **WHEN** 用户角色 cache miss 已经开始为某个用户回源，且该用户的 Add、Remove 或 Replace 用户角色绑定成功后调用 `InvalidateUserRole`
- **THEN** resolver MUST 调用 `Invalidate(userID.String())`，并在方法返回时完成通用 revision 提升与指定 item 删除
- **AND** 失效前开始但失效后完成的旧回源结果 MUST NOT 写入该用户缓存或返回给当前授权 caller
- **AND** localcache 透明重试成功时后续授权 MUST 使用失效后的最终角色集合

#### Scenario: 全量失效抑制所有旧 load 返回与写回

- **WHEN** 一个或多个用户角色 cache miss 已经开始回源，且系统调用 `InvalidateAllUserRoles`
- **THEN** resolver MUST 调用 `InvalidateAll()`，并在方法返回时完成通用 revision 提升与全部 item 删除
- **AND** 全量失效前开始但失效后完成的任一旧回源结果 MUST NOT 写入缓存或返回给授权 caller
- **AND** localcache 透明重试成功时后续授权 MUST 使用全量失效后的最终角色集合

#### Scenario: 连续失效竞态保持 fail-closed

- **WHEN** `RolesForUser` 的首次回源结果因通用 revision 变化被抑制
- **THEN** localcache MUST 透明重试一次且 permission feature MUST NOT 增加 generation-aware retry wrapper
- **WHEN** 第二次回源仍被失效并返回 `ErrInvalidated`
- **THEN** 当前授权请求 MUST fail-closed，MUST NOT 使用旧角色集合产生允许结果
- **AND** loader 错误、context 取消或过期回源结果 MUST NOT 写入缓存

#### Scenario: cache disabled 模式保持直接回源

- **WHEN** `rbac.user_role_cache.enabled=false`
- **THEN** 系统 MUST 不创建通用 loading cache，并逐次从 PostgreSQL 回源当前启用角色
- **AND** `InvalidateUserRole` 与 `InvalidateAllUserRoles` MUST 保持同步幂等且不得引入旧 load 写回路径
- **AND** 回源成功 MUST 返回独立角色 ID slice，回源错误或 context 取消 MUST 保持 fail-closed

