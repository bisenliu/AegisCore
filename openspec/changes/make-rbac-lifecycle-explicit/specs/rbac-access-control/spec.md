## MODIFIED Requirements

### Requirement: 授权热路径用户角色缓存

系统 MUST 使用有容量上限的本地 loading cache 缓存用户当前启用角色 ID 集合，并通过主动失效和全量清空使在线 RBAC 变更及时收敛。关闭缓存 MUST 只影响性能，不得改变授权结果；启用和 disabled 模式 MUST 提供相同的显式幂等关闭契约。

#### Scenario: 缓存命中

- **WHEN** 用户角色缓存命中
- **THEN** 授权 MUST 使用缓存中的角色 ID 副本
- **AND** 调用方对返回 slice 的修改 MUST NOT 污染缓存内部值

#### Scenario: 缓存未命中

- **WHEN** 用户角色缓存未命中
- **THEN** 系统 MUST 合并同一用户的并发回源并查询 PostgreSQL 中的当前启用角色
- **AND** loader 错误 MUST NOT 写入缓存

#### Scenario: 缓存配置和容量

- **WHEN** user-service 装配启用的用户角色缓存
- **THEN** 系统 MUST 从 `rbac.user_role_cache` 读取正值 `size`、`ttl` 和 `load_timeout`
- **AND** 未显式配置时 MUST 使用 `enabled=true`、`size=100000`、`ttl=5s` 和 `load_timeout=500ms`
- **AND** 容量淘汰、准入拒绝或 TTL 过期后 MUST 能通过 PostgreSQL 回源

#### Scenario: 关闭缓存

- **WHEN** `rbac.user_role_cache.enabled` 为 false
- **THEN** `size`、`ttl` 和 `load_timeout` MAY 为零值
- **AND** 系统 MUST 直接回源并保持正确的 fail-closed 授权语义
- **AND** disabled 模式 MUST 提供与启用模式相同的幂等 `Close` 契约

#### Scenario: 显式关闭缓存

- **WHEN** 调用方对启用或 disabled user-role resolver/cache 调用 `Close`
- **THEN** `Close` MUST 是幂等的，并且 MUST NOT 关闭调用方注入的 Redis、Ent 或 PostgreSQL 共享资源
- **AND** 关闭后授权语义 MUST 继续 fail-closed，不得因为关闭本地缓存而产生允许结果

#### Scenario: 在线绑定变更失效

- **WHEN** 在线用户角色添加、替换或移除成功
- **THEN** 本实例 MUST 失效对应用户缓存
- **AND** 其他副本 MUST 通过 policy sync 感知变更并失效缓存

#### Scenario: policy reload 清空缓存

- **WHEN** RBAC policy reload 或全量策略刷新完成
- **THEN** 系统 MUST 清空本实例用户角色缓存
- **AND** 后续请求 MUST 通过回源重新建立本地投影

### Requirement: Policy 加载与多副本同步

系统 MUST 在启动时通过显式初始化入口加载初始 policy，并在在线 RBAC 写操作成功后刷新本实例状态，再通过 Redis policy version、Pub/Sub 和周期性版本补偿同步其他副本。授权热路径 MUST 使用本地 enforcer 和本地用户角色解析结果，MUST NOT 每请求读取 Redis version。

#### Scenario: 启动加载 policy

- **WHEN** user-service 启动 permission/RBAC 模块
- **THEN** composition 层 MUST 显式调用 initial load 初始化入口，并将可取消或带超时的启动 context 传给 policy loader
- **AND** permission infrastructure MUST NOT 通过 `RegisterInitialLoad(fx.Lifecycle, ...)` 或等价 Fx lifecycle adapter 注册初始加载
- **AND** loader MUST 能观察启动超时或取消

#### Scenario: 初始加载失败

- **WHEN** 初始 policy 加载失败或被取消
- **THEN** engine MUST 记录最近错误和 reload 失败指标
- **AND** 后续授权 MUST fail-closed，服务 MUST 保持既有启动语义
- **AND** reload 状态和 readiness 可观测性 MUST 保留失败信息

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

#### Scenario: 其他副本收敛

- **WHEN** watcher 通过 Pub/Sub 或周期性版本检查发现远端 policy version 更新
- **THEN** 系统 MUST reload policy 或失效用户角色缓存
- **AND** Pub/Sub 丢失时周期性版本补偿 MUST 使副本最终收敛

### Requirement: RBAC 分层与组合边界

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL 和 HTTP response 细节 MUST 留在对应 composition、transport 或 infrastructure 边界。permission infrastructure 可以拥有 Ent、Redis、Casbin 和 cache 的具体适配细节，但 MUST NOT 依赖 Fx 或 Dig。

#### Scenario: application 直接构造

- **WHEN** role 或 permission application service 在单元测试或非 Fx 调用方中构造
- **THEN** 调用方 MUST 能以普通强类型参数提供 store、lookup、notifier 和 logger
- **AND** application/domain MUST NOT import Fx、嵌入 `fx.In` 或声明仅服务于 DI 的 tag

#### Scenario: feature composition 组装依赖

- **WHEN** 正式 feature module 注册 application service、policy engine、watcher、cache 和 adapter
- **THEN** 无 DI metadata 的构造器 MUST 直接注册
- **AND** named、optional 或配置转换 adapter MUST 留在 feature composition 边界
- **AND** 必需安全依赖缺失时 graph MUST 构造失败，MUST NOT 静默降级

#### Scenario: 显式生命周期由 composition 编排

- **WHEN** 正式 feature module 需要启动 initial load、RBAC watcher 或关闭 user-role cache
- **THEN** Fx lifecycle hook MUST 只登记在 composition 层，并且 MUST 调用 infrastructure 对象暴露的显式 `Initialize`、`Start`、`Stop` 或 `Close` 方法
- **AND** `user-service/internal/features/permission/infrastructure` 的生产代码 MUST NOT import `go.uber.org/fx` 或 `go.uber.org/dig`，也 MUST NOT 使用 `fx.Lifecycle`、`fx.Hook`、`fx.In` 或 `fx.Out`

#### Scenario: 共享边界保持最小

- **WHEN** role 需要查询权限或 permission 需要解析用户身份
- **THEN** 消费侧 application MUST 定义最小 port 并由相邻 feature 或 integration adapter 实现
- **AND** feature MUST NOT 导入其他 feature 的 infrastructure 或 HTTP transport

#### Scenario: 服务资源归属

- **WHEN** user-service 装配 RBAC 的 PostgreSQL、Redis 和用户角色缓存
- **THEN** 具名资源 MUST 来自服务自有 `resources.postgres` 和 `resources.redis`，feature cache MUST 来自 `rbac.user_role_cache`
- **AND** RBAC MUST NOT 把服务业务配置或 key schema 下沉到 `common`
- **AND** watcher 和 cache 的 `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源

#### Scenario: 架构调整不改变行为

- **WHEN** provider 注册、依赖投影、logger 注入、application 构造方式或显式生命周期编排调整
- **THEN** 权限、角色、绑定、授权、policy reload、缓存失效、跨副本同步和 CLI 行为 MUST 保持不变
- **AND** 架构检查 MUST 阻止 application/domain 引入框架依赖或生产代码重新依赖全局 logger
