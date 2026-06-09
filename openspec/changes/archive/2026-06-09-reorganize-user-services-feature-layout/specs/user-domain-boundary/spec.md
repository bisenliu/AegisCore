## MODIFIED Requirements

### Requirement: Organize user service code by capability boundary
系统 SHALL 将用户服务核心业务代码按 feature capability 聚合到 `user-services/internal/features` 下。用户资料能力 MUST 位于 `user-services/internal/features/user`，认证能力 MUST 位于 `user-services/internal/features/auth`；每个 feature MUST 使用 `api`、`app`、`domain` 和 `store` 子目录表达稳定职责。`api` MUST 承载 HTTP request/response DTO 和 Swagger 文档模型；`app` MUST 承载 use case/service、commands、ports、controller 和用例级 mapper；`domain` MUST 承载实体、枚举、领域错误和领域规则；`store` MUST 承载 DB/Redis adapter，并按 datastore 类型继续细分为 `postgres` 或 `redis`。`bootstrap`、`router` 和 `validators` MUST 保留为服务级进程启动/Fx 装配、HTTP 路由挂载和全局纯函数校验边界。

#### Scenario: Locate user capability code
- **Given** 开发者修改用户创建、查询、列表、用户领域模型、用户错误映射或用户持久化 store
- **When** 代码属于用户资料能力
- **Then** HTTP DTO 和 Swagger 文档模型 MUST 位于 `user-services/internal/features/user/api`
- **Then** 用户 controller、service、commands、ports 和用例级 mapper MUST 位于 `user-services/internal/features/user/app`
- **Then** 用户领域实体、状态枚举、领域错误和领域规则 MUST 位于 `user-services/internal/features/user/domain`
- **Then** Ent/PostgreSQL 用户资料持久化实现 MUST 位于 `user-services/internal/features/user/store/postgres`
- **Then** 实现 MUST NOT 新增横向 `user-services/internal/controller`、`user-services/internal/service`、`user-services/internal/repository`、`user-services/internal/api` 或 `user-services/internal/domain` 包承载用户资料能力代码

#### Scenario: Locate auth capability code
- **Given** 开发者修改登录、刷新、改密、登出、token 会话、认证凭据、token version 或认证会话 store
- **When** 代码属于认证能力
- **Then** HTTP DTO 和 Swagger 文档模型 MUST 位于 `user-services/internal/features/auth/api`
- **Then** 认证 controller、service、commands、ports、凭据校验、token 签发和会话编排 MUST 位于 `user-services/internal/features/auth/app`
- **Then** 认证会话实体、凭据模型、认证领域错误、Redis key 业务语义和认证领域规则 MUST 位于 `user-services/internal/features/auth/domain`
- **Then** Redis 认证会话实现 MUST 位于 `user-services/internal/features/auth/store/redis`
- **Then** 认证凭据和 token version 的 PostgreSQL adapter MUST 位于 `user-services/internal/features/auth/store/postgres`
- **Then** 实现 MUST NOT 新增横向 `user-services/internal/controller`、`user-services/internal/service`、`user-services/internal/repository`、`user-services/internal/api` 或 `user-services/internal/domain` 包承载认证能力代码

#### Scenario: Keep runtime boundaries outside feature folders
- **Given** 开发者修改进程启动、Fx/DI 装配、基础设施 provider、Gin engine、HTTP server 生命周期、路由挂载或 Swagger 路由
- **When** 代码属于服务运行时而非单一业务能力
- **Then** 代码 MUST 保持在 `user-services/internal/bootstrap` 或 `user-services/internal/router`
- **Then** 业务 feature 目录 MUST NOT 承载通用服务启动生命周期逻辑
- **Then** `bootstrap` MAY 依赖 feature 的 app/store provider 完成 Fx 装配，但 feature store MUST NOT 反向依赖 `bootstrap`

#### Scenario: Keep validators as global pure function boundary
- **Given** 开发者修改用户服务特定的请求清洗、基础校验、解析或转换函数
- **When** 该逻辑不访问 datastore、不依赖 Gin、不执行业务编排且可作为纯函数复用
- **Then** 代码 MAY 保持在 `user-services/internal/validators`
- **Then** validators MUST NOT 导入 feature app service、feature store、Gin、Ent、Redis 或外部 datastore client
- **Then** validators MAY 导入 feature api DTO、feature domain 枚举和共享校验/响应原语

#### Scenario: Preserve external contracts after feature layout migration
- **Given** 用户服务业务代码已迁移到 `user-services/internal/features`
- **When** 调用方访问现有用户资料或认证会话 API
- **Then** HTTP 路径、认证边界、响应信封、公开 JSON 字段和错误码 MUST 保持兼容
- **Then** 配置 YAML key、`AEGISCORE_` 环境变量覆盖、Redis key 格式、PostgreSQL/Redis 命名实例和 Fx named injection MUST 保持不变
- **Then** 数据库 schema、Atlas migration、Ent 生成代码和 Go module 边界 MUST 保持不变
