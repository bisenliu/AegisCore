# user-domain-boundary

## Purpose

用户领域边界能力定义 Service、Repository、Ent 持久化模型与领域实体之间的依赖边界，确保用户状态规则和对外业务数据由领域模型承载，持久化实现细节不泄漏到上层。
## Requirements
### Requirement: Organize user service code by capability boundary
系统 SHALL 将用户服务核心业务代码按 feature capability 聚合到 `user-services/internal/features` 下。用户资料能力 MUST 位于 `user-services/internal/features/user`，认证能力 MUST 位于 `user-services/internal/features/auth`；每个 feature MUST 使用 `api`、`app`、`domain`、`transport/http` 和 `infra` 子目录表达稳定职责。`api` MUST 承载 HTTP request/response DTO 和 Swagger 文档模型；`app` MUST 承载 use case/service、commands/queries、ports、credential/token/session 组件、应用结果类型和用例级 mapper；`domain` MUST 承载实体、枚举、领域错误和领域规则；`transport/http` MUST 承载 Gin controller、HTTP route registration、HTTP request validation/normalization、HTTP DTO 映射、HTTP 错误映射和 HTTP handler 测试；`infra` MUST 承载 DB/Redis adapter，并按 datastore 类型继续细分为 `postgres` 或 `redis`。每个 feature 根目录 MUST 提供 `module.go` 暴露 Fx module 用于装配本 feature 内部 provider。`bootstrap` MUST 保留为服务级进程启动、基础设施 runtime 和 HTTP 总装边界；服务级路由入口 MAY 保留系统路由和 Swagger 路由，但业务 endpoint 注册 MUST 由 feature-local `transport/http` 包拥有。用户服务 MUST NOT 继续使用全局 `internal/validators` 承载 user/auth HTTP DTO 清洗规则。

#### Scenario: Locate user capability code
- **Given** 开发者修改用户创建、查询、列表、用户领域模型、用户错误映射或用户持久化 adapter
- **When** 代码属于用户资料能力
- **Then** HTTP DTO 和 Swagger 文档模型 MUST 位于 `user-services/internal/features/user/api`
- **Then** 用户 service、commands、queries、ports、应用结果类型和用例级 mapper MUST 位于 `user-services/internal/features/user/app`
- **Then** 用户 Gin controller、HTTP routes、HTTP validation、HTTP DTO 映射、HTTP 错误映射和 controller tests MUST 位于 `user-services/internal/features/user/transport/http`
- **Then** 用户领域实体、状态枚举、领域错误和领域规则 MUST 位于 `user-services/internal/features/user/domain`
- **Then** Ent/PostgreSQL 用户资料持久化实现 MUST 位于 `user-services/internal/features/user/infra/postgres`
- **Then** 用户 feature Fx module MUST 位于 `user-services/internal/features/user/module.go`
- **Then** 实现 MUST NOT 新增横向 `user-services/internal/controller`、`user-services/internal/service`、`user-services/internal/repository`、`user-services/internal/api` 或 `user-services/internal/domain` 包承载用户资料能力代码

#### Scenario: Locate auth capability code
- **Given** 开发者修改登录、刷新、改密、登出、token 会话、认证凭据、token version 或认证会话 adapter
- **When** 代码属于认证能力
- **Then** HTTP DTO 和 Swagger 文档模型 MUST 位于 `user-services/internal/features/auth/api`
- **Then** 认证 service、commands、ports、应用结果类型、凭据校验、token 签发和会话编排 MUST 位于 `user-services/internal/features/auth/app`
- **Then** 认证 Gin controller、HTTP routes、HTTP validation、HTTP DTO 映射、HTTP 错误映射和 controller tests MUST 位于 `user-services/internal/features/auth/transport/http`
- **Then** 认证会话实体、凭据模型、认证领域错误、Redis key 业务语义和认证领域规则 MUST 位于 `user-services/internal/features/auth/domain`
- **Then** Redis 认证会话实现 MUST 位于 `user-services/internal/features/auth/infra/redis`
- **Then** 认证凭据和 token version 的 PostgreSQL adapter MUST 位于 `user-services/internal/features/auth/infra/postgres`
- **Then** 认证 feature Fx module MUST 位于 `user-services/internal/features/auth/module.go`
- **Then** 实现 MUST NOT 新增横向 `user-services/internal/controller`、`user-services/internal/service`、`user-services/internal/repository`、`user-services/internal/api` 或 `user-services/internal/domain` 包承载认证能力代码

#### Scenario: Keep runtime boundaries outside feature folders
- **Given** 开发者修改进程启动、Fx app 创建、共享配置、Zap logger、timezone、validation module、具名 PostgreSQL/Redis/Ent runtime、Gin engine、HTTP server 生命周期、系统路由或 Swagger 路由
- **When** 代码属于服务运行时而非单一业务能力
- **Then** 代码 MUST 保持在 `user-services/internal/bootstrap` 或服务级 runtime/router 边界
- **Then** 业务 feature 目录 MUST NOT 承载通用服务启动生命周期逻辑
- **Then** `bootstrap` MUST 通过 feature `Module` 和 feature HTTP route registration 完成总装
- **Then** feature `infra`、`app`、`domain` 和 `transport/http` 包 MUST NOT 反向依赖 `bootstrap`

#### Scenario: Keep app free of HTTP transport details
- **Given** 开发者修改用户或认证 app service、commands、queries、ports、mapper、credential、token 或 session 组件
- **When** 代码位于 `user-services/internal/features/<feature>/app`
- **Then** app 包 MUST NOT 导入 Gin、HTTP binder、HTTP response writer、`common/http/ginvalidation`、`common/contract/response`、feature `api` 或 feature `transport/http`
- **Then** app 包 MUST 通过 command/query、domain model、应用结果类型、消费侧 ports、领域错误、应用错误分类和 common 安全原语表达用例执行
- **Then** 认证能力中跨 credential、token、session、service 和 transport 共同消费的稳定失败语义 MUST 位于 `auth/domain`
- **Then** HTTP DTO 到 command/query 的映射 MUST 发生在 `transport/http` controller 中
- **Then** app 结果到 HTTP response DTO、分页响应契约和 HTTP 应用错误的映射 MUST 发生在 `transport/http` 中

#### Scenario: Keep app service outputs transport neutral
- **Given** 用户或认证 app service 完成 use case 执行
- **When** service 方法返回结果给调用方
- **Then** 返回类型 MUST 是领域模型、应用结果类型或其他 transport-neutral 类型
- **Then** 返回类型 MUST NOT 是 `api/*Response`、`response.Envelope`、`response.PaginatedData`、`response.Pagination` 或其他 HTTP 响应 DTO
- **Then** service 错误 MUST 使用领域错误、应用错误分类或普通 Go error 表达
- **Then** service MUST NOT 构造 `response.BadRequestError`、`response.UnauthenticatedError`、`response.TokenInvalidError`、`response.ConflictError`、`response.NotFoundError`、`response.FromError` 或其他 HTTP 响应错误

#### Scenario: Preserve external contracts after feature layout migration
- **Given** 用户服务业务代码已迁移到 `user-services/internal/features/<feature>/{api,app,domain,transport/http,infra}`
- **When** 调用方访问现有用户资料或认证会话 API
- **Then** HTTP 路径、认证边界、响应信封、公开 JSON 字段和错误码 MUST 保持兼容
- **Then** 配置 YAML key、`AEGISCORE_` 环境变量覆盖、Redis key 格式、PostgreSQL/Redis 命名实例和 Fx named injection MUST 保持不变
- **Then** 数据库 schema、Atlas migration、Ent 生成代码和 Go module 边界 MUST 保持不变

### Requirement: User domain model owns user state rules
系统 SHALL 在 `user-services/internal/features/user/domain` 中提供用户领域实体，用于表达用户对外身份、基础资料、认证凭据摘要、状态、token version 和公开时间戳。用户状态相关业务判断、状态合法性校验、允许值列表以及 JSON/query 文本解析 MUST 通过用户领域实体或用户状态枚举方法表达，app service 层 MUST NOT 直接依赖 Ent 用户模型字段实现用户状态规则。用户 HTTP API DTO MAY 直接复用用户领域状态枚举作为请求和响应中的状态类型；实现 MUST NOT 在 `user-services/internal/features/user/api` 中重复定义用户状态类型、状态常量或状态解析/枚举校验方法。

#### Scenario: Domain user represents service user data needs
- **Given** Service 层需要处理用户查询、创建响应、登录、改密或 token version 相关流程
- **When** Service 从用户 Repository 获取用户数据
- **Then** Repository MUST 返回用户领域实体
- **Then** 用户领域实体 MUST 包含 Service 当前业务所需的 `user_id`、`nickname`、`username`、`password_hash`、`status`、`token_version`、`created_at` 和 `updated_at`
- **Then** Service MUST NOT 为读取这些字段导入 Ent 用户模型

#### Scenario: User state rules are centralized
- **Given** 登录或改密流程需要判断用户是否正常、禁用或必须修改密码
- **When** Service 执行用户状态判断
- **Then** Service MUST 使用用户领域实体或 `user.UserStatus` 提供的方法表达状态规则
- **Then** Service MUST NOT 通过散落的 Ent 字段类型转换重复实现相同状态规则

#### Scenario: API DTO reuses domain user status enum
- **Given** 用户创建或用户列表 HTTP request DTO 需要表达可选 `status` 字段
- **When** DTO 定义状态字段类型
- **Then** DTO MUST 直接复用 `user-services/internal/features/user/domain.UserStatus`
- **Then** `user-services/internal/features/user/api` MUST NOT 重复定义 `UserStatus` 类型、状态常量、`IsValid`、`AllowedValues`、`UnmarshalText` 或 `UnmarshalJSON`
- **Then** 共享 enum 校验和 Gin 绑定 MUST 继续使用领域状态类型上的解析和枚举方法

### Requirement: Persistence models remain store implementation details
系统 SHALL 将 Ent 用户模型限制在 PostgreSQL store 实现边界内。用户资料和认证能力的 store port 抽象 MUST 面向领域模型和领域错误，不得向 Service、Controller 或 DTO 层暴露 Ent 生成类型。

#### Scenario: Postgres store maps Ent user to domain user
- **Given** PostgreSQL store 使用 Ent client 查询或创建用户记录
- **When** Store 方法需要返回用户数据给 Service
- **Then** PostgreSQL store MUST 将 Ent 用户模型转换为用户领域实体或认证所需的裁剪凭据模型
- **Then** Ent 用户模型 MUST NOT 作为 store port 方法返回类型

#### Scenario: Store preserves domain errors
- **Given** Ent 查询返回 not found 或唯一约束冲突
- **When** PostgreSQL store 处理该错误
- **Then** Store MUST 继续返回 `user.ErrUserNotFound` 或 `user.ErrUserAlreadyExists`
- **Then** Store MUST NOT 构造 `common/contract/response` 应用错误

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
系统 SHALL 将 Ent predicate 构建细节封装在 PostgreSQL infra adapter 内部实现目录中，例如 `user-services/internal/features/user/infra/postgres/predicates.go`。App service 层 MUST NOT import `user-services/ent/user`、`user-services/ent/predicate` 或其他 ORM 查询构造细节；所有查询条件拼装 MUST 由 infra adapter 内部根据业务语义完成。

#### Scenario: Build list predicates inside postgres infra
- **Given** 用户列表查询包含 status、username、nickname 或软删除过滤条件
- **When** PostgreSQL infra adapter 查询用户列表
- **Then** infra adapter 内部 MUST 根据业务 query/input 构造 Ent predicates
- **Then** 对外端口 MUST 暴露 List、Find、Exists 或 Count 这类业务语义方法，而不是 predicate 拼装能力

#### Scenario: Reject Ent predicate usage in service
- **Given** 用户 service 需要查询指定状态的用户
- **When** 开发者准备在 `service.go` 中 import `user-services/ent/user` 并调用 `user.StatusEQ`
- **Then** 实现 MUST 被视为违反领域边界
- **Then** service MUST 改为通过业务 query/input 调用 app port，由 PostgreSQL infra adapter 内部构造 predicate

### Requirement: Ent schema defaults remain independent from domain layer

系统 SHALL 将 Ent schema 的数据库默认值声明限制在 schema 层或更低层的持久化契约内。Ent schema MUST NOT 为声明数据库字段默认值导入 `user-services/internal/features/user/domain`；业务状态规则 MUST 继续由领域模型和 store 映射边界承载。

#### Scenario: User status default is declared without domain import
- **Given** `User` Ent schema 需要为 `status` 字段声明数据库默认值
- **When** Ent codegen 或 Atlas schema source 编译 `user-services/ent/schema/`
- **Then** `User` Ent schema MUST NOT 导入 `user-services/internal/features/user/domain`
- **Then** `status` 字段默认值 MUST 保持为数据库契约值 `100`
- **Then** Service 层状态判断 MUST 继续通过 `userdomain.UserStatus` 或用户领域实体表达

#### Scenario: Store remains the domain mapping boundary
- **Given** PostgreSQL store 从 Ent 用户模型读取 `status` 数值
- **When** Store 返回用户数据给 Service
- **Then** Store MUST 将 Ent `status` 数值转换为 `userdomain.UserStatus`
- **Then** Ent schema 本地默认值常量 MUST NOT 成为 Service 或 Controller 的业务状态规则来源
