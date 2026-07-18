## MODIFIED Requirements

### Requirement: Casbin 授权与 HTTP 边界

系统 MUST 在认证通过后使用 RBAC 授权中间件保护权限、角色和用户业务接口。授权 MUST 使用用户与角色的稳定 subject、Gin route template object 和 HTTP method action，并在任何身份、策略或执行异常下 fail-closed。

#### Scenario: 构造授权请求

- **WHEN** 已认证请求进入受 RBAC 保护的 `/api/v1` 路由
- **THEN** 中间件 MUST 使用请求上下文中的用户 ID、Gin `FullPath()` 和 HTTP method 构造授权请求
- **AND** 用户 subject MUST 使用 `user:<user_uuid>`，角色 subject MUST 使用 `role:<role_uuid>`

#### Scenario: 允许和拒绝访问

- **WHEN** 用户当前启用角色拥有匹配 HTTP method 和 route template 的启用权限
- **THEN** 系统 MUST 允许请求进入目标 controller
- **AND** 没有匹配权限时系统 MUST 返回禁止访问错误

#### Scenario: 认证 subject 无效

- **WHEN** 请求缺少用户 ID、用户 ID 类型非法或 subject 不能解析为用户 UUID
- **THEN** 系统 MUST 返回未认证错误并拒绝请求
- **AND** 系统 MUST NOT 调用底层 Casbin engine

#### Scenario: 授权执行异常

- **WHEN** 用户角色回源失败、context 取消或 Casbin 执行返回错误
- **THEN** 系统 MUST 拒绝请求并返回内部错误
- **AND** 系统 MUST NOT 将执行异常折叠为允许结果

#### Scenario: policy 未加载时安全拒绝

- **WHEN** 初始 policy 未加载或最近一次 reload 失败导致本地 enforcer 不可用
- **THEN** 系统 MUST fail-closed 并拒绝受 RBAC 保护的请求
- **AND** 系统 MUST NOT 因 policy 缺失产生允许结果
- **AND** readiness/startup MUST 暴露 policy 不可用状态以阻止接入业务流量

#### Scenario: 公开和预检请求旁路

- **WHEN** 请求命中显式授权白名单或使用 `OPTIONS` 方法
- **THEN** 中间件 MUST 允许请求继续处理并 MUST NOT 调用授权服务

#### Scenario: policy 权威来源

- **WHEN** policy loader 构造授权策略
- **THEN** policy MUST 从启用角色、启用权限、角色权限绑定和用户角色绑定派生
- **AND** 独立 `casbin_rules` 表 MUST NOT 成为业务权威来源，用户身份解析 MUST 排除已软删除用户

#### Scenario: 超级管理员通配授权

- **WHEN** 用户拥有 `internal/shared/rbacbaseline` 定义的内置超级管理员角色
- **THEN** policy loader MUST 为该角色提供受保护业务接口的 wildcard policy
- **AND** 超级管理员角色常量 MUST 只由 `rbacbaseline` 提供

#### Scenario: 路由注册安全依赖完整

- **WHEN** user-service 注册 `/api/v1` 权限、角色和用户业务路由
- **THEN** 这些路由 MUST 经过当前认证和 RBAC 授权中间件链
- **AND** token version validator、RBAC authorizer 或必需 controller 缺失时系统 MUST 拒绝注册部分业务路由

### Requirement: Policy 加载与多副本同步

系统 MUST 在启动时通过显式 fail-closed 初始化入口加载初始 policy，并在在线 RBAC 写操作成功后刷新本实例状态，再通过 Redis policy version、Pub/Sub 和周期性版本补偿同步其他副本。授权热路径 MUST 使用本地 enforcer 和本地用户角色解析结果，MUST NOT 每请求读取 Redis version。

#### Scenario: 启动加载 policy

- **WHEN** user-service 启动 permission/RBAC 模块
- **THEN** composition 层 MUST 显式调用表达 fail-closed 降级启动语义的 initial load 初始化入口，并将可取消或带超时的启动 context 传给 policy loader
- **AND** permission infrastructure MUST NOT 通过 `RegisterInitialLoad(fx.Lifecycle, ...)` 或等价 Fx lifecycle adapter 注册初始加载
- **AND** loader MUST 能观察启动超时或取消
- **AND** composition 层 MUST NOT 保留看似严格启动但不会因初始 reload 失败触发的 error branch

#### Scenario: 初始加载失败

- **WHEN** 初始 policy 加载失败或被取消
- **THEN** engine MUST 记录最近错误和 reload 失败指标
- **AND** 后续授权 MUST fail-closed，`app.Start` MUST 保持成功
- **AND** reload 状态和 readiness/startup 可观测性 MUST 保留失败信息并拒绝接入业务流量

#### Scenario: 后续 reload 恢复 readiness

- **WHEN** 初始 policy 加载失败后，后续 Pub/Sub、版本补偿或显式 reload 成功加载当前 policy
- **THEN** engine MUST 替换为最新可用 policy 并清除最近 reload 错误
- **AND** readiness/startup MUST 恢复成功
- **AND** 后续授权 MUST 按最新 policy 判定允许或拒绝

#### Scenario: 在线权限或角色变更

- **WHEN** 权限、角色状态或角色权限绑定通过在线 API 持久化成功
- **THEN** 本实例 MUST reload policy 并发布新的 Redis policy version 和 Pub/Sub 通知

#### Scenario: 在线用户角色变更

- **WHEN** 用户角色绑定通过在线 API 持久化成功
- **THEN** 本实例 MUST 失效相关用户缓存并通知其他副本刷新授权投影

#### Scenario: 同步失败向调用方传播

- **WHEN** 持久化成功后的 reload、缓存失效、version 发布或通知失败
- **THEN** command service MUST 向调用方返回同步错误，成功响应 MUST NOT 掩盖该错误
- **AND** `PolicyChangeNotifier` MUST 是正式 command service 的必需依赖

#### Scenario: reload 和发布局部失败

- **WHEN** policy refresh coordinator 执行本地 reload 和 version 发布
- **THEN** 本地 reload 失败后系统 MUST 仍尝试发布 version
- **AND** 两者同时失败时返回错误 MUST 保留两项失败
- **AND** 只有两者均成功时系统才 MUST 标记本实例已应用该 version

#### Scenario: watcher 生命周期

- **WHEN** user-service 启停 Redis policy watcher
- **THEN** `NewWatcher` MUST 只构造 watcher 对象，MUST NOT 启动 goroutine、订阅 Redis 或执行版本补偿循环
- **AND** `Start()` MUST 是幂等的，并使用内部可取消 context 驱动长期循环
- **AND** `Stop(ctx)` MUST 是幂等的，取消内部 context，并在调用方 context 限制内等待循环退出

#### Scenario: watcher 停止超时

- **WHEN** 调用方传入的 `Stop(ctx)` context 在 watcher 循环退出前到期
- **THEN** `Stop(ctx)` MUST 返回 context 相关错误
- **AND** watcher MUST 保持后续重复 `Stop(ctx)` 可安全调用，且 MUST NOT 关闭调用方注入的 Redis、Ent 或 PostgreSQL 共享资源

#### Scenario: RBAC lifecycle 停止合并清理错误

- **WHEN** RBAC lifecycle 停止时 watcher stop 和 user-role cache close 同时失败
- **THEN** 单个 lifecycle hook 返回的错误 MUST 同时保留 watcher stop error 和 cache close error
- **AND** 调用方 MUST 能通过标准错误链匹配两个 cause
- **AND** cache close MUST 在 watcher stop 返回错误时仍被执行

#### Scenario: 其他副本收敛

- **WHEN** watcher 通过 Pub/Sub 或周期性版本检查发现远端 policy version 更新
- **THEN** 系统 MUST reload policy 或失效用户角色缓存
- **AND** Pub/Sub 丢失时周期性版本补偿 MUST 使副本最终收敛
