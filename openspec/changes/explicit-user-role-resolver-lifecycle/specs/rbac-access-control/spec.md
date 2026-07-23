## MODIFIED Requirements

### Requirement: RBAC 架构、装配与资源生命周期

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。permission application MUST 只保留权限查询、授权、policy loading/sync 和 seed/角色绑定所需最小端口，不得保留公开权限 command 或仅服务于公开 route diff 的生产装配。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL、HTTP response 和 named resource metadata MUST 留在对应 composition、transport 或 infrastructure 边界。RBAC 自有 watcher、cache、user-role resolver 和 policy 投影资源 MUST 显式启动、停止和回滚。

#### Scenario: 分层和最小依赖

- **WHEN** role 或 permission application service 在单元测试或非 Fx 调用方中构造
- **THEN** 调用方 MUST 能以普通强类型参数提供 store、lookup、notifier 和 logger
- **AND** application/domain MUST NOT import Fx、嵌入 `fx.In` 或声明仅服务于 DI 的 tag
- **AND** 消费侧 application MUST 定义最小 port 并由相邻 feature 或 integration adapter 实现，feature MUST NOT 导入其他 feature 的 infrastructure 或 HTTP transport
- **AND** role 仍使用的 permission lookup MUST NOT 因删除 permission command 而被移除

#### Scenario: bootstrap application 和 store 边界

- **WHEN** 实现超级管理员 bootstrap 应用服务
- **THEN** 服务 MUST 位于 `user-service/internal/features/role/application/bootstrap/`，并通过最小 `BootstrapStore` port 调用持久化能力
- **AND** application 层 MUST 负责校验和归一化输入、校验 bootstrap 密码策略、哈希密码、使用固定 bootstrap user ID 和固定 super admin role ID
- **AND** application 层 MUST NOT 导入 Ent predicate、HTTP transport、Gin、Fx、SQL、Redis 或 datastore concrete implementation
- **AND** PostgreSQL adapter MUST 位于 role infrastructure 边界并只实现 bootstrap application 拥有的最小 port

#### Scenario: framework-neutral adapter 和 composition 边界

- **WHEN** 构造 role store、permission store、policy loader、Casbin engine、Redis policy store、watcher、cache 或 adapter
- **THEN** constructor MUST 接收普通强类型参数或无 DI metadata 的 options
- **AND** constructor MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag、named result 或 group result
- **AND** 具名 `primary_db`、`cache_redis`、optional、group 或生命周期依赖选择 MUST 留在 feature composition 边界
- **AND** public provider MUST 只暴露 controller、authorizer、route registrar、health/status 和 application port 等稳定 contract，父 module MUST NOT 消费 feature infrastructure concrete implementation

#### Scenario: 有状态资源单实例多视图

- **WHEN** composition 需要同时提供 authorization、policy reload、policy health、policy store 或 publisher 等接口视图
- **THEN** composition MUST 为同一有状态 adapter 构造一个实例并通过普通 Go 赋值暴露所需端口
- **AND** 系统 MUST NOT 为不同接口视图重复构造有状态 engine、store、version tracker、watcher 或 cache

#### Scenario: 必需同步依赖不可降级

- **WHEN** 角色、角色权限或用户角色写侧服务装配完成
- **THEN** 服务 MUST 具备可用的 policy change notifier
- **AND** 缺少 notifier 或其他必需安全 collaborator 时 constructor MUST 返回明确 error 并拒绝装配，MUST NOT panic
- **AND** 系统 MUST NOT 以 no-op、nil fallback 或兼容 wrapper 静默跳过 policy reload、Redis policy version 或 watcher 同步语义

#### Scenario: watcher、cache 和 lifecycle

- **WHEN** user-service 启停 Redis policy watcher 和 user-role resolver/cache
- **THEN** `NewWatcher` MUST 只构造 watcher 对象，MUST NOT 启动 goroutine、订阅 Redis 或执行版本补偿循环
- **AND** user-role resolver/cache 的 Fx result MUST 显式提供同时具备 `Start(context.Context) error` 与 `Close() error` 的 lifecycle 视图，lifecycle hook MUST NOT 通过关闭接口的 type assertion 探测启动能力
- **AND** lifecycle hook MUST 在启动阶段先调用 user-role resolver/cache 的 `Start(ctx)`，再执行初始 policy 加载并启动 watcher
- **AND** user-role resolver/cache 启动失败时 MUST 返回启动错误，且 MUST NOT 执行初始 policy 加载或启动 watcher
- **AND** `Start()` 和 `Stop(ctx)` MUST 幂等，`Stop(ctx)` MUST 取消内部 context，并在调用方 context 限制内等待循环退出
- **AND** `Stop(ctx)` 超时时 MUST 返回 context 相关错误，并保持后续重复停止安全
- **AND** 启动失败或服务停止时已启动 watcher MUST 被停止，已创建 cache MUST 幂等关闭
- **AND** watcher stop 和 cache close 同时失败时单个 lifecycle hook MUST 保留全部 cause，且 cache close MUST 在 watcher stop 返回错误时仍被执行

#### Scenario: 共享资源所有权和 fail-closed

- **WHEN** RBAC 关闭 watcher、cache、store 或 resolver
- **THEN** `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源
- **AND** 关闭后授权语义 MUST 继续 fail-closed，不得因为本地资源不可用而产生允许结果
- **AND** RBAC MUST NOT 把服务业务配置、权限基线或 key schema 下沉到 `common`
