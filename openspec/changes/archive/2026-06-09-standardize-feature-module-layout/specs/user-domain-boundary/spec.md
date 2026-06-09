## MODIFIED Requirements

### Requirement: Organize user service code by capability boundary
系统 SHALL 将用户服务核心业务代码按 feature capability 聚合到 `user-services/internal/features` 下。用户资料能力 MUST 位于 `user-services/internal/features/user`，认证能力 MUST 位于 `user-services/internal/features/auth`；每个 feature MUST 使用 `api`、`app`、`domain`、`transport/http` 和 `infra` 子目录表达稳定职责。`api` MUST 承载 HTTP request/response DTO 和 Swagger 文档模型；`app` MUST 承载 use case/service、commands/queries、ports、credential/token/session 组件和用例级 mapper；`domain` MUST 承载实体、枚举、领域错误和领域规则；`transport/http` MUST 承载 Gin controller、HTTP route registration、HTTP request validation/normalization 和 HTTP handler 测试；`infra` MUST 承载 DB/Redis adapter，并按 datastore 类型继续细分为 `postgres` 或 `redis`。每个 feature 根目录 MUST 提供 `module.go` 暴露 Fx module 用于装配本 feature 内部 provider。`bootstrap` MUST 保留为服务级进程启动、基础设施 runtime 和 HTTP 总装边界；服务级路由入口 MAY 保留系统路由和 Swagger 路由，但业务 endpoint 注册 MUST 由 feature-local `transport/http` 包拥有。用户服务 MUST NOT 继续使用全局 `internal/validators` 承载 user/auth HTTP DTO 清洗规则。

#### Scenario: Locate user capability code
- **Given** 开发者修改用户创建、查询、列表、用户领域模型、用户错误映射或用户持久化 adapter
- **When** 代码属于用户资料能力
- **Then** HTTP DTO 和 Swagger 文档模型 MUST 位于 `user-services/internal/features/user/api`
- **Then** 用户 service、commands、queries、ports 和用例级 mapper MUST 位于 `user-services/internal/features/user/app`
- **Then** 用户 Gin controller、HTTP routes、HTTP validation 和 controller tests MUST 位于 `user-services/internal/features/user/transport/http`
- **Then** 用户领域实体、状态枚举、领域错误和领域规则 MUST 位于 `user-services/internal/features/user/domain`
- **Then** Ent/PostgreSQL 用户资料持久化实现 MUST 位于 `user-services/internal/features/user/infra/postgres`
- **Then** 用户 feature Fx module MUST 位于 `user-services/internal/features/user/module.go`
- **Then** 实现 MUST NOT 新增横向 `user-services/internal/controller`、`user-services/internal/service`、`user-services/internal/repository`、`user-services/internal/api` 或 `user-services/internal/domain` 包承载用户资料能力代码

#### Scenario: Locate auth capability code
- **Given** 开发者修改登录、刷新、改密、登出、token 会话、认证凭据、token version 或认证会话 adapter
- **When** 代码属于认证能力
- **Then** HTTP DTO 和 Swagger 文档模型 MUST 位于 `user-services/internal/features/auth/api`
- **Then** 认证 service、commands、ports、凭据校验、token 签发和会话编排 MUST 位于 `user-services/internal/features/auth/app`
- **Then** 认证 Gin controller、HTTP routes、HTTP validation 和 controller tests MUST 位于 `user-services/internal/features/auth/transport/http`
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
- **Then** app 包 MUST NOT 导入 Gin、HTTP binder、HTTP response writer、`common/http/ginvalidation` 或 feature `transport/http`
- **Then** app 包 MUST 通过 command/query、domain model、消费侧 ports 和 common 安全原语表达用例执行
- **Then** HTTP DTO 到 command/query 的映射 MUST 发生在 `transport/http` controller 中

#### Scenario: Preserve external contracts after feature layout migration
- **Given** 用户服务业务代码已迁移到 `user-services/internal/features/<feature>/{api,app,domain,transport/http,infra}`
- **When** 调用方访问现有用户资料或认证会话 API
- **Then** HTTP 路径、认证边界、响应信封、公开 JSON 字段和错误码 MUST 保持兼容
- **Then** 配置 YAML key、`AEGISCORE_` 环境变量覆盖、Redis key 格式、PostgreSQL/Redis 命名实例和 Fx named injection MUST 保持不变
- **Then** 数据库 schema、Atlas migration、Ent 生成代码和 Go module 边界 MUST 保持不变

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
