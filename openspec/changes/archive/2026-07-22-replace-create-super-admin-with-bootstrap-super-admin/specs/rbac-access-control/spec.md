## MODIFIED Requirements

### Requirement: RBAC 系统数据与运维 CLI

系统 MUST 提供带服务上下文的 `rbac seed` 和一次性 `rbac bootstrap-super-admin` 命令，用于维护系统角色、代码定义权限投影、默认绑定和全新数据库的首次超级管理员引导。系统角色与默认绑定 MUST 由 seed port 根据 `internal/shared/rbacbaseline` 写入，全部权限定义 MUST 只来自 `rbacbaseline.DefaultPermissions()`。RBAC 运维 CLI MUST 通过 `aegiscore-user-service` 根命令调用，旧 `aegiscore-user-services` 根命令 MUST NOT 作为 RBAC 兼容入口保留。

#### Scenario: 初始化系统基线

- **WHEN** 运维执行 `aegiscore-user-service rbac seed`
- **THEN** 系统 MUST 幂等创建或更新基线角色、权限投影和绑定并输出变更统计
- **AND** 系统角色 MUST 保持系统数据标记，Permission MUST NOT 包含 `Active` 或 `IsSystem`
- **AND** seed MUST NOT 创建业务用户、自动分配超级管理员或自动删除基线之外的权限记录
- **AND** 非 seed 的公开 HTTP 路径 MUST NOT 创建、修改或启停权限

#### Scenario: 一次性超级管理员引导输入

- **WHEN** 运维执行 `aegiscore-user-service rbac bootstrap-super-admin --username <name> --nickname <nickname> --password-env <env>`
- **THEN** 系统 MUST 要求 `--username` 必填且无默认值，并将 username trim 后转为小写
- **AND** `--nickname` MUST 可选，trim 后为空时 MUST 使用归一化 username
- **AND** `--password-env` 默认 MUST 为 `ADMIN_BOOTSTRAP_PASSWORD`，密码 MUST 只从该环境变量读取
- **AND** 密码 MUST NOT trim，首尾空格 MUST 作为密码内容参与校验和哈希
- **AND** 密码长度 MUST 为 12 至 72 字节
- **AND** 命令行 MUST NOT 提供直接传递密码、user ID、reset、force、reuse 或 reactivate 的参数

#### Scenario: 固定 bootstrap 标识

- **WHEN** 系统执行超级管理员首次引导
- **THEN** 系统 MUST 使用 `user-service/internal/features/role/application/bootstrap.BootstrapSuperAdminUserID` 作为固定用户 ID，值为 `00000000-0000-0000-0000-000000000002`
- **AND** 固定用户 ID MUST 由代码定义，MUST NOT 通过 CLI 参数、环境变量或配置覆盖
- **AND** 系统 MUST 使用 `rbacbaseline.SuperAdminRoleID` 作为超级管理员角色 ID

#### Scenario: 事务性首次引导

- **WHEN** bootstrap store 执行首次超级管理员引导
- **THEN** 系统 MUST 在同一个 PostgreSQL 事务中获取固定 transaction advisory lock、查询超级管理员角色、检查固定用户 ID、检查 username、创建固定 ID 用户并创建用户角色绑定
- **AND** 超级管理员角色 MUST 存在、`is_system=true` 且 `active=true`
- **AND** 固定用户 ID 查询 MUST 包含软删除用户，MUST NOT 添加 `deleted_at IS NULL`
- **AND** username 占用检查 MUST 覆盖正常用户和软删除用户
- **AND** bootstrap 用户状态 MUST 为 `identity.UserStatusMustChangePassword`
- **AND** 用户密码 MUST 使用应用层传入的 bcrypt hash
- **AND** 任一步失败 MUST 回滚整个事务，不得留下已创建但没有角色的用户、已绑定角色但用户状态错误的记录或只有用户没有完整 bootstrap 结果的部分状态
- **AND** 唯一约束冲突 MUST 映射成稳定应用错误，MUST NOT 直接暴露 Ent 或 PostgreSQL 错误

#### Scenario: 重复执行拒绝

- **WHEN** 数据库中存在固定 `BootstrapSuperAdminUserID` 对应用户
- **THEN** `rbac bootstrap-super-admin` MUST 拒绝执行并返回非零退出码
- **AND** 稳定公开错误消息 MUST 为 `super admin bootstrap has already been completed`
- **AND** 无论该用户是否软删除、是否禁用、是否仍有超级管理员角色或 username 是否被修改，系统都 MUST 视为已完成引导
- **AND** 命令 MUST NOT 尝试修复、复用、重置或重新绑定该用户

#### Scenario: 后续超级管理员授权

- **WHEN** 首次 bootstrap 已完成且需要授权其他超级管理员
- **THEN** 系统 MUST 通过现有在线用户角色绑定 API 完成授权
- **AND** 在线流程 MUST 负责权限校验、policy version 发布和缓存收敛
- **AND** 系统 MUST NOT 允许再次运行 bootstrap 创建其他超级管理员
- **AND** 系统 MUST NOT 提供离线密码重置、离线超级管理员恢复或 `recover-super-admin` 入口
- **AND** 如果所有超级管理员均不可用，本方案 MUST 只允许 DBA 人工介入或重新初始化数据库

#### Scenario: 删除旧超级管理员命令

- **WHEN** 运维或测试调用 `rbac create-super-admin`、`rbac assign-super-admin`、`--reset-password` 或 `ADMIN_RESET_PASSWORD`
- **THEN** 系统 MUST 拒绝或忽略旧入口，使旧命令和旧 flag 无法作为公开 CLI 调用
- **AND** 系统 MUST NOT 保留旧命令别名、双版本 CLI 共存或旧数据自动恢复行为

#### Scenario: 离线命令不等同在线刷新

- **WHEN** HTTP 副本运行期间执行 seed 或 bootstrap-super-admin
- **THEN** 命令 MUST 只修改持久化数据并 MUST NOT 宣称已触发运行期 policy refresh
- **AND** 运维 MUST 滚动重启副本或触发在线 RBAC 刷新使运行实例收敛

### Requirement: RBAC 架构、装配与资源生命周期

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。permission application MUST 只保留权限查询、授权、policy loading/sync 和 seed/角色绑定所需最小端口，不得保留公开权限 command 或仅服务于公开 route diff 的生产装配。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL、HTTP response 和 named resource metadata MUST 留在对应 composition、transport 或 infrastructure 边界。RBAC 自有 watcher、cache 和 policy 投影资源 MUST 显式启动、停止和回滚。

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

- **WHEN** user-service 启停 Redis policy watcher 和 user-role cache
- **THEN** `NewWatcher` MUST 只构造 watcher 对象，MUST NOT 启动 goroutine、订阅 Redis 或执行版本补偿循环
- **AND** `Start()` 和 `Stop(ctx)` MUST 幂等，`Stop(ctx)` MUST 取消内部 context，并在调用方 context 限制内等待循环退出
- **AND** `Stop(ctx)` 超时时 MUST 返回 context 相关错误，并保持后续重复停止安全
- **AND** 启动失败或服务停止时已启动 watcher MUST 被停止，已创建 cache MUST 幂等关闭
- **AND** watcher stop 和 cache close 同时失败时单个 lifecycle hook MUST 保留全部 cause，且 cache close MUST 在 watcher stop 返回错误时仍被执行

#### Scenario: 共享资源所有权和 fail-closed

- **WHEN** RBAC 关闭 watcher、cache、store 或 resolver
- **THEN** `Stop` 或 `Close` MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源
- **AND** 关闭后授权语义 MUST 继续 fail-closed，不得因为本地资源不可用而产生允许结果
- **AND** RBAC MUST NOT 把服务业务配置、权限基线或 key schema 下沉到 `common`
