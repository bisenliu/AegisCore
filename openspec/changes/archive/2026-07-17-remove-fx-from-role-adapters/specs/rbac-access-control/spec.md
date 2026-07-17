## MODIFIED Requirements

### Requirement: RBAC 分层与组合边界

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL 和 HTTP response 细节 MUST 留在对应 composition、transport 或 infrastructure 边界。role infrastructure store constructor MUST 使用显式普通 Go 参数接收 Ent client 和必要的消费侧窄 port，MUST NOT 通过 `fx.In`、`dig.In`、`fx.Out`、`dig.Out`、`name` tag 或其他 DI metadata 表达依赖。

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

#### Scenario: 共享边界保持最小

- **WHEN** role 需要查询权限或 permission 需要解析用户身份
- **THEN** 消费侧 application MUST 定义最小 port 并由相邻 feature 或 integration adapter 实现
- **AND** feature MUST NOT 导入其他 feature 的 infrastructure 或 HTTP transport

#### Scenario: 服务资源归属

- **WHEN** user-service 装配 RBAC 的 PostgreSQL、Redis 和用户角色缓存
- **THEN** 具名资源 MUST 来自服务自有 `resources.postgres` 和 `resources.redis`，feature cache MUST 来自 `rbac.user_role_cache`
- **AND** RBAC MUST NOT 把服务业务配置或 key schema 下沉到 `common`

#### Scenario: 架构调整不改变行为

- **WHEN** provider 注册、依赖投影、logger 注入或 application 构造方式调整
- **THEN** 权限、角色、绑定、授权、policy reload、缓存失效、跨副本同步和 CLI 行为 MUST 保持不变
- **AND** 架构检查 MUST 阻止 application/domain 引入框架依赖或生产代码重新依赖全局 logger
