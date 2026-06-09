## ADDED Requirements

### Requirement: Organize user service code by capability boundary

系统 SHALL 将用户服务核心业务代码按能力聚合。用户资料能力 MUST 位于 `user-services/internal/user`，认证能力 MUST 位于 `user-services/internal/auth`；每个能力目录 SHOULD 聚合本能力的 controller、service、ports、model、commands、api DTO、adapter 和 store adapter。`bootstrap`、`router` 和 `validators` MAY 保留为服务级运行时、路由注册和纯校验边界。

#### Scenario: Locate user capability code
- **Given** 开发者修改用户创建、查询、列表、用户领域模型、用户错误映射或用户持久化 store
- **When** 代码属于用户资料能力
- **Then** 相关代码 MUST 位于 `user-services/internal/user` 或其子目录
- **Then** Ent/PostgreSQL 用户持久化实现 MUST 位于 `user-services/internal/user/store/postgres`

#### Scenario: Locate auth capability code
- **Given** 开发者修改登录、刷新、改密、登出、token 会话或认证会话 store
- **When** 代码属于认证能力
- **Then** 相关代码 MUST 位于 `user-services/internal/auth` 或其子目录
- **Then** Redis 认证会话实现 MUST 位于 `user-services/internal/auth/store/redis`

#### Scenario: Keep runtime boundaries outside capability folders
- **Given** 开发者修改 Fx 组装、Gin engine、HTTP server 生命周期、路由挂载或 Swagger 路由
- **When** 代码属于服务运行时而非单一业务能力
- **Then** 代码 MUST 保持在 `user-services/internal/bootstrap` 或 `user-services/internal/router`
- **Then** 业务能力目录 MUST NOT 承载通用服务启动生命周期逻辑

### Requirement: Define ports by service use case ownership

系统 SHALL 要求每个能力的 `ports.go` 围绕本能力 `service.go` 的最小依赖面定义接口。Ports 属于调用方 service 的用例需求，而不是由适配器能力倒推；接口 MUST 只暴露当前 use case 必需方法，MUST NOT 将 store 的完整 CRUD 能力整体搬入 ports。

#### Scenario: Define minimal user store port
- **Given** 用户 service 只需要按用户名读取凭据或按用户 ID 读取基础资料
- **When** 开发者定义 `ports.go`
- **Then** 端口 MUST 只声明当前 use case 需要的方法
- **Then** 端口 MUST NOT 预先暴露 Create、Update、Delete、List 等无关方法

#### Scenario: Keep infrastructure details out of ports
- **Given** service 需要访问 PostgreSQL 或 Redis-backed 数据
- **When** 开发者定义 service 消费的端口接口
- **Then** 接口 MUST 使用业务语义输入和领域模型
- **Then** 接口 MUST NOT 暴露 Ent client、Redis client、SQL builder、Ent predicate 或基础设施错误细节

### Requirement: Keep capability adapters thin

系统 SHALL 允许能力通过 `adapter.go` 向其他能力暴露极简读取或协作接口，但 adapter MUST 仅做字段裁剪、结果映射或轻量封装。复杂业务逻辑、跨多个依赖的编排逻辑、事务逻辑和策略决策逻辑 MUST 保留在 `service.go`；adapter MUST NOT 为配合调用方需求污染本能力领域模型。

#### Scenario: Expose basic user info to auth
- **Given** 认证能力只需要用户 ID 和状态等极少量用户信息
- **When** 用户能力提供 `AuthUserAdapter`
- **Then** adapter MAY 通过用户 service 或最小 port 读取用户模型并返回裁剪后的 `AuthUserInfo`
- **Then** adapter MUST NOT 在 `adapter.go` 中实现复杂业务编排

#### Scenario: Reject adapter as workflow layer
- **Given** 跨能力协作需要多个依赖、事务或策略决策
- **When** 开发者准备把流程写入 `adapter.go`
- **Then** 实现 MUST 拒绝该设计
- **Then** 业务编排 MUST 回到所属能力的 `service.go` 或明确的新 use case 中

### Requirement: Encapsulate Ent predicates in store implementation

系统 SHALL 将 Ent predicate 构建细节封装在 store 内部实现目录中，例如 `user-services/internal/user/store/postgres/predicates.go`。Service 层 MUST NOT import `user-services/ent/user`、`user-services/ent/predicate` 或其他 ORM 查询构造细节；所有查询条件拼装 MUST 由 store 内部根据业务语义完成。

#### Scenario: Build list predicates inside postgres store
- **Given** 用户列表查询包含 status、username、nickname 或软删除过滤条件
- **When** PostgreSQL store 查询用户列表
- **Then** store 内部 MUST 根据业务 query/input 构造 Ent predicates
- **Then** 对外端口 MUST 暴露 List、Find、Exists 或 Count 这类业务语义方法，而不是 predicate 拼装能力

#### Scenario: Reject Ent predicate usage in service
- **Given** 用户 service 需要查询指定状态的用户
- **When** 开发者准备在 `service.go` 中 import `user-services/ent/user` 并调用 `user.StatusEQ`
- **Then** 实现 MUST 被视为违反领域边界
- **Then** service MUST 改为通过业务 query/input 调用 store 端口，由 store 内部构造 predicate
