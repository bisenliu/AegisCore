## MODIFIED Requirements

### Requirement: Casbin 授权与 HTTP 边界

系统 MUST 在认证通过后使用 RBAC 授权中间件保护权限、角色和用户业务接口。授权 MUST 使用用户与角色的稳定 subject、Gin route template object 和 HTTP method action，并在任何身份、策略或执行异常下 fail-closed。RBAC 授权 HTTP 边界 MUST 通过稳定 authorizer 和 route registrar contract 暴露给 service composition，MUST NOT 要求父 module 注入 permission feature 的内部 concrete implementation。

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

- **WHEN** policy 未加载、用户角色回源失败、context 取消或 Casbin 执行返回错误
- **THEN** 系统 MUST 拒绝请求并返回内部错误
- **AND** 系统 MUST NOT 将执行异常折叠为允许结果

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
- **AND** token version validator、RBAC authorizer 或必需 route registrar 缺失时系统 MUST 拒绝注册部分业务路由

#### Scenario: 授权边界不暴露 concrete

- **WHEN** service composition 注入 RBAC authorizer 或 authorized route registrar
- **THEN** 注入类型 MUST 是 permission feature 暴露的 public contract
- **AND** service composition MUST NOT 依赖 permission Casbin engine、policy loader、Redis store、version tracker 或 watcher 的 concrete implementation

### Requirement: RBAC 分层与组合边界

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL 和 HTTP response 细节 MUST 留在对应 composition、transport 或 infrastructure 边界。role infrastructure store constructor MUST 使用显式普通 Go 参数接收 Ent client 和必要的消费侧窄 port，MUST NOT 通过 `fx.In`、`dig.In`、`fx.Out`、`dig.Out`、`name` tag 或其他 DI metadata 表达依赖。permission infrastructure 可以拥有 Ent、Redis、Casbin 和 cache 的具体适配细节，但 MUST NOT 依赖 Fx 或 Dig。feature Fx module MUST 使用 internal/public provider 边界收缩内部 implementation 可见性，父 module MUST NOT 消费 feature 内部 concrete implementation。

#### Scenario: application 直接构造

- **WHEN** role 或 permission application service 在单元测试或非 Fx 调用方中构造
- **THEN** 调用方 MUST 能以普通强类型参数提供 store、lookup、notifier 和 logger
- **AND** application/domain MUST NOT import Fx、嵌入 `fx.In` 或声明仅服务于 DI 的 tag

#### Scenario: feature composition 组装依赖

- **WHEN** 正式 feature module 注册 application service、policy engine、watcher、cache 和 adapter
- **THEN** 无 DI metadata 的构造器 MUST 直接注册
- **AND** named、optional 或配置转换 adapter MUST 留在 feature composition 边界
- **AND** 必需安全依赖缺失时 graph MUST 构造失败，MUST NOT 静默降级

#### Scenario: role store adapter 显式构造

- **WHEN** 调用方构造 `RoleStore`、`RolePermissionStore` 或 `UserRoleStore`
- **THEN** 调用方 MUST 以普通 Go 参数显式传入 `*ent.Client`
- **AND** constructor MUST NOT 暴露或接收 `fx.In`、`dig.In`、`fx.Out`、`dig.Out` 或 `name:"primary_db"` 等 DI metadata
- **AND** `PermissionLookup` 等跨 feature 依赖 MUST 继续通过 role application 消费侧窄 port 显式注入，MUST NOT 扩大为 permission infrastructure 宽接口

#### Scenario: role feature 禁止 Fx/Dig 回归

- **WHEN** 执行 `user-service-architecture-lint`
- **THEN** lint MUST 检查 `user-service/internal/features/role` 的 domain、application、infrastructure 和 transport 生产 Go 文件
- **AND** 这些文件 MUST NOT import `go.uber.org/fx` 或 `go.uber.org/dig`
- **AND** 这些文件 MUST NOT 使用 `fx.In`、`fx.Out`、`dig.In`、`dig.Out` 或仅服务于 DI 的 tag
- **AND** role feature 的 `fx.go` 与 `fx_test.go` MAY 继续作为 composition 和 graph 验证边界使用 Fx

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

- **WHEN** provider 注册、依赖投影、logger 注入、application 构造方式、route registrar 或显式生命周期编排调整
- **THEN** 权限、角色、绑定、授权、policy reload、缓存失效、跨副本同步和 CLI 行为 MUST 保持不变
- **AND** 架构检查 MUST 阻止 application/domain 引入框架依赖、生产代码重新依赖全局 logger 或父 module 消费 feature infrastructure concrete

#### Scenario: internal provider 不跨 module 可见

- **WHEN** feature module 装配 store、engine、watcher、tracker、cache holder 或 metrics implementation 等内部 implementation
- **THEN** 这些 implementation MUST 通过 `fx.Private` 或等价 provider 边界限制在所属 feature module 内部
- **AND** public provider MUST 只暴露 controller、authorizer、route registrar、health/status 和 application port 等稳定 contract
- **AND** 不再需要跨 module concrete 注入时 MUST 删除 `fx.As(fx.Self())` 暴露

### Requirement: Permission adapter 显式装配边界

permission 的 PostgreSQL、Redis 和 Casbin infrastructure adapter 构造 API MUST 使用普通 Go 参数表达必需依赖，并 MUST NOT 在 adapter constructor 中暴露或要求 Fx/Dig metadata。生产 Fx composition MAY 在 feature composition 边界选择具名资源和生命周期挂钩，但 MUST 通过显式 Go 赋值暴露 concrete 与 application/authorization port 视图。跨 module 公开视图 MUST 限定为稳定 public contract，MUST NOT 继续公开 permission infrastructure concrete 给父 module。

#### Scenario: adapter constructor 不携带 DI metadata

- **WHEN** 构造 permission `PermissionStore`、policy `Loader`、Casbin `Engine` 或 Redis policy `Store`
- **THEN** constructor MUST 接收普通强类型参数或无 DI metadata 的 options
- **AND** constructor MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag、`fx.As`、`fx.Self`、named result 或 group result

#### Scenario: composition 显式选择服务资源

- **WHEN** 正式 permission Fx module 装配 PostgreSQL、Redis、policy loader、policy store、version tracker 或 authorization engine
- **THEN** 具名 `primary_db`、`cache_redis` 或生命周期依赖的选择 MUST 留在 `features/permission/fx.go` composition 边界
- **AND** PostgreSQL、Redis 和 Casbin adapter package 的生产构造 API MUST NOT import Fx 或 Dig 只为读取这些 tags

#### Scenario: 同一 Engine 暴露多个端口

- **WHEN** composition 需要同时提供 authorization、policy reload 和 policy health 视图
- **THEN** composition MUST 构造一个 `Engine` 实例并通过普通 Go 赋值暴露这些端口
- **AND** 系统 MUST NOT 为 authorization port、reload port 或 health port 重复构造有状态 `Engine`
- **AND** 父 module MUST NOT 直接注入 concrete `Engine`

#### Scenario: 同一 Redis Store 暴露发布端口

- **WHEN** composition 需要同时提供 Redis policy store 内部视图和 `permissionapplication.PolicyVersionPublisher` 等接口视图
- **THEN** composition MUST 构造一个 `Store` 实例并通过普通 Go 赋值暴露这些端口
- **AND** 系统 MUST NOT 为内部 concrete 和 interface 视图重复构造有状态 Redis policy store 或 version tracker
- **AND** 父 module MUST NOT 直接注入 Redis policy store、version tracker 或 watcher concrete

#### Scenario: 行为保持不变

- **WHEN** permission adapter 构造 API 从 Fx/Dig metadata 改为普通 Go 参数，或 Fx provider 边界从 concrete self 暴露改为 private/internal 暴露
- **THEN** 权限目录、route diff、Casbin policy、授权 fail-closed、Redis policy version、Pub/Sub、用户角色缓存失效和多副本同步语义 MUST 保持不变
- **AND** 本变更 MUST NOT 迁移 Casbin initial load、watcher `Start/Stop` 或用户角色缓存 `Close` 生命周期
