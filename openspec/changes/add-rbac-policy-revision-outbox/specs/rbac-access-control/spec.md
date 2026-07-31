## ADDED Requirements

### Requirement: RBAC policy revision 与事务 outbox

系统 MUST 以 PostgreSQL 中追加式 `rbac_policy_revisions` 记录作为在线 RBAC policy revision 的唯一权威来源，并为每次成功的在线角色、角色权限或用户角色 mutation 持久化一条全局单调 revision 和一条唯一 pending outbox event。业务 mutation、revision 分配和 outbox 写入 MUST 在同一 PostgreSQL transaction 中提交；Redis MUST NOT 分配、替代或恢复权威 revision。

#### Scenario: 在线 mutation 原子提交 revision 与 outbox

- **WHEN** 角色创建、角色更新、角色启停、角色权限添加/替换/删除或用户角色添加/替换/删除成功提交
- **THEN** 数据库 MUST 同时存在对应业务变更、一条已提交 policy revision 和一条引用该 revision 的 pending outbox event
- **AND** 不同成功 mutation 的 revision MUST 全局唯一且按数值单调递增
- **AND** revision 序列 MAY 因事务回滚存在空洞，调用方 MUST NOT 假设 revision 连续

#### Scenario: transaction 任一步失败时完整回滚

- **WHEN** 业务 mutation、revision 插入、outbox 插入或 transaction commit 任一步失败
- **THEN** 系统 MUST 返回错误并回滚该 transaction 内全部业务、revision 和 outbox 写入
- **AND** 系统 MUST NOT 执行本地 reload、缓存失效、Redis version 写入或 Pub/Sub 发布

#### Scenario: 校验失败不分配 revision

- **WHEN** 在线写请求因输入非法、对象不存在、对象不可用、系统角色保护、绑定冲突或其他业务校验失败而未产生 mutation
- **THEN** 系统 MUST NOT 创建 policy revision 或 outbox event
- **AND** 已有业务关系和授权投影 MUST 保持不变

#### Scenario: outbox event 契约

- **WHEN** 系统为已提交 revision 创建 outbox event
- **THEN** event MUST 包含稳定 event ID、唯一 revision、`kind`、`reason`、相关 `role_id`/`user_id`/`permission_id`、`status`、`attempt_count`、`next_attempt_at`、`last_error`、唯一幂等键、`created_at`、`updated_at` 和 `delivered_at`
- **AND** 新 event 的 `status` MUST 为 `pending`、`attempt_count` MUST 为零、`delivered_at` MUST 为空
- **AND** event kind MUST 区分全局 `policy_changed` 与定向 `user_role_changed`，幂等键 MUST 能由 revision 稳定确定

#### Scenario: 即时同步失败后保留恢复事实

- **WHEN** PostgreSQL transaction 已提交但本地 reload、缓存失效、Redis version 写入或 Pub/Sub 发布失败
- **THEN** 对应 revision 和 pending outbox event MUST 保持已提交且不得被删除、回滚或标记为已投递
- **AND** API MAY 返回同步错误，但 mutation 的可靠恢复 MUST NOT 依赖该次 Redis 操作成功

#### Scenario: 离线写入边界

- **WHEN** RBAC seed、bootstrap 或受控 migration 修改离线系统数据
- **THEN** 本 change MUST NOT 要求这些离线流程伪装成在线 outbox dispatcher 或宣称已完成副本同步
- **AND** 运维 MUST 继续通过显式 reload 或滚动重启使授权投影收敛

## MODIFIED Requirements

### Requirement: Casbin 授权、策略缓存与多副本同步

系统 MUST 在认证后使用 RBAC 中间件保护权限、角色和用户业务接口，并以 PostgreSQL 关系数据作为业务权威投影，以本地 Casbin policy 和用户角色 loading cache 作为授权投影。授权 MUST 使用稳定 subject、Gin route template 和 HTTP method，并在任何身份、策略或执行异常下 fail-closed。系统 MUST 在启动时显式加载 policy；在线写 MUST 先原子提交数据库 policy revision 与 outbox，再使用该 revision 刷新本实例并通过 Redis 缓存和 Pub/Sub 加速通知其他副本。Redis MUST NOT 作为 policy version 的权威来源或 revision 分配器；授权热路径 MUST 使用本地 enforcer 和本地用户角色解析结果，MUST NOT 每请求读取 Redis version。

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
- **WHEN** 用户角色回源失败、context 取消、Casbin 执行错误、policy 未加载或最近一次 reload 失败
- **THEN** 系统 MUST 拒绝请求并暴露 policy 不可用 readiness/startup 状态，MUST NOT 将异常或 policy 缺失折叠为允许结果
- **WHEN** 请求命中显式授权白名单或使用 `OPTIONS`
- **THEN** 中间件 MUST 允许请求并 MUST NOT 调用授权服务
- **WHEN** 注册 `/api/v1` 权限、角色和用户业务路由
- **THEN** 这些路由 MUST 经过当前认证和 RBAC 中间件链；token version validator、RBAC authorizer 或必需 route registrar 缺失时系统 MUST 拒绝注册部分路由

#### Scenario: policy 权威来源、初始加载与恢复

- **WHEN** policy loader 构造授权策略
- **THEN** policy MUST 从启用角色、角色权限绑定、permissions 投影和用户角色绑定派生
- **AND** loader MUST NOT 使用权限 active predicate，独立 `casbin_rules` 表 MUST NOT 成为业务权威来源，用户身份解析 MUST 排除已软删除用户
- **WHEN** user-service 启动 permission/RBAC 模块
- **THEN** composition 层 MUST 显式调用初始加载入口，并将可取消或带超时的启动 context 传给 loader
- **WHEN** 初始加载失败或被取消
- **THEN** engine MUST 记录最近错误和 reload 失败指标，后续授权 MUST fail-closed，`app.Start` MUST 保持成功
- **AND** reload 状态和 readiness/startup MUST 保留失败信息并拒绝接入业务流量
- **WHEN** 后续 Pub/Sub、版本补偿或显式 reload 成功
- **THEN** engine MUST 替换为最新 policy、清除最近错误并恢复 readiness/startup

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
- **THEN** 系统 MUST 直接回源、返回独立角色 ID slice 并保持 fail-closed；direct stats source MUST 使用 `LoadSuccess` 与 `LoadError` 表达逐次结果

#### Scenario: 在线写后同步与离线变更

- **WHEN** 角色、角色权限或用户角色 mutation 通过在线 API 持久化成功
- **THEN** 同一 PostgreSQL transaction MUST 已提交对应 policy revision 与 pending outbox event
- **AND** 本实例 MUST 使用已提交 revision reload policy 或失效相关用户缓存，并将同一 revision 写入 Redis cache 和 Pub/Sub 通知
- **AND** reload、缓存失效、Redis 写入或通知失败 MUST 向调用方返回同步错误，成功响应 MUST NOT 掩盖错误；已提交 outbox MUST 保留可恢复事实
- **AND** `PolicyChangeNotifier` MUST 消费 store 返回的数据库 revision，MUST NOT 自行分配 revision
- **WHEN** 权限投影由离线 migration、seed 或 bootstrap 改变
- **THEN** 离线命令 MUST NOT 宣称已完成在线 policy refresh，运维 MUST 显式 reload 或滚动重启副本

#### Scenario: reload 发布与副本收敛

- **WHEN** coordinator 使用已提交数据库 revision 执行本地 reload 和 Redis 发布
- **THEN** 本地 reload 失败后系统 MUST 仍尝试发布该 revision
- **AND** 两者同时失败时错误 MUST 保留两项失败，只有本地应用和发布均成功时系统才 MUST 标记本实例已应用该 revision
- **WHEN** watcher 通过 Pub/Sub 或周期性版本检查发现更大的远端 revision
- **THEN** 系统 MUST reload policy 或失效用户角色缓存
- **AND** Pub/Sub 丢失时周期性版本补偿 MUST 使副本最终收敛，Redis 中的缓存值 MUST NOT 被视为数据库权威 revision 的替代品

### Requirement: RBAC policy sync 兼容 Redis Cluster

RBAC policy sync MUST 兼容 Redis Cluster。数据库 policy revision 写入 Redis cache 时使用的 key、policy refresh channel、周期性版本补偿和 Pub/Sub watcher MUST 使用稳定 hash tag 或 Cluster-compatible key schema，并只消费 Cluster-capable Redis client 或最小接口，MUST NOT 要求 `*redis.Client` 单机 concrete type。Redis MUST 只缓存和传播调用方提供的数据库 revision，MUST NOT 使用 counter 命令分配权威版本。

#### Scenario: 数据库 revision 发布与补偿

- **WHEN** 在线 RBAC 写操作提交新的数据库 policy revision
- **THEN** Redis version key MUST 位于稳定 hash tag 下，并允许 Redis Cluster client 写入该 revision
- **AND** adapter MUST NOT 使用 `INCR`、时间戳或本地计数器生成 revision，较小 revision MUST NOT 覆盖 Redis 中已存在的较大 revision
- **AND** 本地 reload、revision 发布和周期性 version check 的错误语义 MUST 保持 fail-closed、可恢复与可诊断

#### Scenario: Pub/Sub 通知与 watcher 生命周期

- **WHEN** watcher 订阅 policy refresh channel 或接收远端更新
- **THEN** channel 名称 MUST 使用稳定 hash tag 或 Cluster-compatible 命名
- **AND** watcher 停止、cache 关闭或 RBAC runtime 关闭 MUST NOT 关闭共享 Redis Cluster client
