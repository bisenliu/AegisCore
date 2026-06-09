## Why

当前 `user-services/internal/features` 已经按用户资料和认证会话聚合，但 HTTP controller 仍位于 `app`，基础设施 adapter 仍以 `store` 命名，服务内 validators 仍是全局包，运行时仍由全局 router/bootstrap 直接装配所有 feature 细节。随着后续 user、store、finance、payment、iam、audit 等大模块扩展，需要把 feature 内部边界定稿为更清晰、更可扩展的结构。

## What Changes

- **BREAKING（内部 Go import 路径）**：将用户服务 feature 目录定稿为 `api/`、`app/`、`domain/`、`transport/http/`、`infra/postgres/`、`infra/redis/` 和 `module.go`。
- 将用户和认证 controller 从 `features/<feature>/app/controller.go` 迁移到 `features/<feature>/transport/http/controller.go`，使 `app/` 只承载业务用例、commands/queries、ports、service、credential/token/session 组件和 mapper。
- 将 `features/<feature>/store/postgres` 与 `features/<feature>/store/redis` 迁移为 `features/<feature>/infra/postgres` 与 `features/<feature>/infra/redis`，避免与未来门店业务 `store` capability 产生语义冲突。
- 将 `user-services/internal/validators/{user,auth}.go` 下沉到对应 feature 的 `transport/http/validation.go`，使 HTTP DTO 清洗、绑定后的输入处理和 transport-safe 校验归属入口边界。
- 由每个 feature 的 `transport/http/routes.go` 注册自身 HTTP 路由；全局运行时只负责创建 `/api/v1`、公开认证分组、受保护认证分组和用户资源分组的总装。
- 由每个 feature 的 `module.go` 暴露 Fx module，封装本 feature 内部 app service、transport controller 和 infra adapter provider；bootstrap 组合根只引入 `auth.Module`、`user.Module` 以及服务级基础设施。
- 更新文档、capability map、OpenSpec 主规格和测试路径引用，使新分层成为后续开发的事实来源。
- 保持现有 HTTP 路径、请求/响应 JSON、响应信封、错误码、认证边界、配置 key、Redis key、Ent schema、Atlas migration、Go workspace/module 边界不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-domain-boundary`：将 feature 内部分层从 `api/app/domain/store` 调整为 `api/app/domain/transport/http/infra`，并明确各层依赖允许与禁止项。
- `user-service-validation-boundary`：将用户服务特定 HTTP 请求清洗和基础校验从全局 `internal/validators` 下沉到 feature-local `transport/http/validation.go`。
- `request-validation`：补充共享请求校验通过后的 feature-local HTTP validation 边界，保持 common 校验核心与 Gin adapter 不承载用户服务特定规则。
- `http-service-runtime`：将路由注册和 Fx provider 组合调整为 feature-owned routes/module，由 bootstrap 只做运行时总装。
- `user-profile-query`：将用户资料查询的数据访问实现路径从 `features/user/store/postgres` 更新为 `features/user/infra/postgres`，保持 service 端口和外部 API 行为不变。
- `user-profile-create`：将用户创建的数据访问实现路径从 `features/user/store/postgres` 更新为 `features/user/infra/postgres`，保持创建语义和响应契约不变。
- `user-list-query`：将用户列表查询的数据访问实现路径从 `features/user/store/postgres` 更新为 `features/user/infra/postgres`，保持分页、过滤和响应契约不变。
- `user-session-control`：将认证凭据、token version 和 session adapter 路径从 `features/auth/store/*` 更新为 `features/auth/infra/*`，保持登录、刷新、改密和登出语义不变。
- `api-swagger-documentation`：将 Swagger 注解所在 controller 包迁移到 feature-local `transport/http`，继续引用 feature `api` DTO 并保持生成文档兼容。

## Impact

- 代码影响：`user-services/internal/features/user`、`user-services/internal/features/auth`、`user-services/internal/bootstrap`、`user-services/internal/router`、`user-services/internal/validators` 及相关测试、Swagger 注解、文档路径引用会调整。
- API 影响：不改变 `/healthz`、`/swagger/*`、`/api/v1/auth/*` 或 `/api/v1/users*` 的路径、HTTP 方法、认证要求、响应信封、错误码或公开 JSON 字段。
- 数据影响：不修改 Ent schema、Ent 生成代码、Atlas migration、PostgreSQL 表结构、Redis key 格式或 token/session 兼容语义。
- 运行时影响：Fx 装配会从 bootstrap 直接提供所有 feature internals，调整为 bootstrap 引入 feature modules；声明的 `cache_redis`、`user_db`、`common_db` 运行时依赖保持不变。
- 文档影响：`AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 和相关 OpenSpec 主规格需要同步反映新结构。
