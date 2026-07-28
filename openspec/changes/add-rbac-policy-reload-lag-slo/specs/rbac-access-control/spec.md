## MODIFIED Requirements

### Requirement: Casbin 授权、策略缓存与多副本同步

系统 MUST 在认证后使用 RBAC 中间件保护权限、角色和用户业务接口，并以 PostgreSQL 关系数据作为业务权威投影，以本地 Casbin policy 和用户角色 loading cache 作为授权投影。授权 MUST 使用稳定 subject、Gin route template 和 HTTP method，并在任何身份、策略或执行异常下 fail-closed。系统 MUST 在启动时显式加载 policy，在线写成功后刷新本实例，并通过 Redis policy version、Pub/Sub 和周期性版本补偿同步其他副本；Redis 和 watcher 正常运行时，在线 RBAC 写成功后的其他副本 policy 最终生效延迟 MUST 不超过 30 秒；授权热路径 MUST 使用本地 enforcer 和本地用户角色解析结果，MUST NOT 每请求读取 Redis version。

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

- **WHEN** 角色状态、角色权限或用户角色绑定通过在线 API 持久化成功
- **THEN** 本实例 MUST reload policy 或失效相关用户缓存，并发布新的 Redis policy version 和 Pub/Sub 通知
- **AND** reload、缓存失效、version 发布或通知失败 MUST 向调用方返回同步错误，成功响应 MUST NOT 掩盖错误
- **AND** `PolicyChangeNotifier` MUST 是正式 command service 的必需依赖
- **WHEN** 权限投影由离线 migration、seed 或 bootstrap 改变
- **THEN** 离线命令 MUST NOT 宣称已完成在线 policy refresh，运维 MUST 显式 reload 或滚动重启副本

#### Scenario: reload 发布与副本收敛

- **WHEN** coordinator 执行本地 reload 和 version 发布
- **THEN** 本地 reload 失败后系统 MUST 仍尝试发布 version
- **AND** 两者同时失败时错误 MUST 保留两项失败，只有两者均成功时系统才 MUST 标记本实例已应用该 version
- **WHEN** watcher 通过 Pub/Sub 或周期性版本检查发现远端 version 更新
- **THEN** 系统 MUST reload policy 或失效用户角色缓存
- **AND** Pub/Sub 丢失时周期性版本补偿 MUST 使副本最终收敛
- **AND** Redis 和 watcher 正常运行时，其他副本 MUST 在 30 秒内应用最新 policy version 或通过 lag 告警暴露未收敛状态

#### Scenario: policy reload lag 计算

- **WHEN** watcher 成功读取 Redis 最新 policy version 或收到远端 policy refresh payload
- **THEN** 系统 MUST 计算 `max(remote_policy_version - local_applied_policy_version, 0)` 作为当前实例 RBAC policy reload lag
- **AND** watcher 成功应用远端 version 后 MUST 更新本地 applied version 并将该 version 对应的 lag 收敛为 `0`
- **WHEN** Redis policy version 读取失败
- **THEN** 系统 MUST 记录 watcher version check failure，MUST NOT 将 lag 重置为 `0` 或伪装为已收敛
