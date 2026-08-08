## MODIFIED Requirements

### Requirement: RBAC 架构装配与资源生命周期

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。permission application MUST 只保留权限查询、授权、policy loading/sync 和 seed/角色绑定所需最小端口，不得保留公开权限 command 或 route diff 生产装配。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL、HTTP response 和 named resource metadata MUST 留在对应边界。RBAC watcher、Casbin Engine policy reload flight 和 policy 投影主动资源 MUST 显式启动、停止和回滚；无后台执行的 user-role localcache MUST NOT 拥有启停或关闭生命周期。permission composition MUST 以单一 runtime 聚合对象表达稳定组件集合。

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

#### Scenario: watcher、Casbin Engine 与 cache lifecycle

- **WHEN** user-service 启停 permission/RBAC runtime
- **THEN** `NewWatcher` MUST 只构造对象，MUST NOT 启动 goroutine、订阅 Redis 或执行补偿循环
- **AND** hook MUST 先启动 Casbin Engine lifecycle root，再使用启动 context 执行 fail-closed 初始 policy 加载，然后启动 watcher；`Start()` 和 `Stop(ctx)` MUST 幂等，`Stop(ctx)` MUST 取消内部 context 并在调用方期限内等待 watcher 退出
- **AND** Stop 超时 MUST 返回 context 错误并保持重复停止安全，启动失败或停止时已启动 watcher MUST 被停止
- **WHEN** user-role localcache 被构造或应用停止
- **THEN** cache MUST 作为无后台 goroutine 的普通对象使用，Fx result 与 hook MUST NOT 为其提供或调用 `Start(context.Context) error`、`Close() error`、closed state 或 lifecycle rollback

#### Scenario: Casbin Engine shared reload flight root

- **WHEN** Casbin Engine 启动后创建 shared reload flight
- **THEN** `startFlightLocked` MUST 从 engine lifecycle root 派生 shared flight context，MUST NOT 从 `context.Background()` 或任一 waiter context 派生 shared flight context
- **AND** 任一单个 `ReloadToRevision(ctx)` 或 `RefreshToRevision(ctx)` waiter context 取消 MUST 只取消该调用等待，MUST NOT 取消其他 waiter 仍需要的 shared reload flight
- **AND** 全部 waiter 取消后 engine MAY 取消当前 shared reload flight 并保持 fail-closed，后续新 waiter MUST 能启动 fresh flight
- **WHEN** engine lifecycle root 被 RBAC lifecycle Stop、启动回滚或服务 shutdown 取消
- **THEN** 正在阻塞的 shared reload loader MUST 收到取消信号，engine MUST 记录 reload 失败并保持 fail-closed，MUST NOT 使用旧 `context.Background()` 兼容路径继续执行该 flight
- **WHEN** `InitializeFailClosed(ctx)` 执行启动期初始加载
- **THEN** engine MUST 使用 target revision 0 执行 reload；若启动 context 取消、loader 失败或目标 revision 未达到，服务启动 MUST 保持成功且后续授权 MUST fail-closed

#### Scenario: 共享资源所有权与关闭安全

- **WHEN** RBAC 关闭 watcher、store、Casbin Engine 或其他主动资源
- **THEN** `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源
- **AND** 关闭后授权 MUST 继续 fail-closed，不得因本地资源不可用产生允许结果
- **AND** RBAC MUST NOT 把服务业务配置、权限基线、角色值复制或 key schema 下沉到 `common`
