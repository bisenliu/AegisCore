# AegisCore Agent Guide

本文件是 AI 代理和协作者的仓库入口规则。结构规则以本文件和 `docs/ARCHITECTURE.md` 为准；本仓库不再维护 OpenSpec/OPSX 工件，不要重新新增 `openspec/` 或 `docs/opsx/`。

## 1. Quick Start

- 架构与分层规则：`docs/ARCHITECTURE.md`
- 开发说明：`docs/DEVELOPMENT.md`
- 产品上下文：`docs/PRODUCT.md`
- 测试说明：`docs/TESTING.md`
- Lint 自动化：`docs/GO_LINT_AUTOMATION.md`

## 2. Repository Shape

- `go.work`：Go workspace，包含 `common` 和 `user-service` 两个模块。
- `common/`：跨服务稳定契约和基础能力，按 `contract`、`runtime`、`http`、`security`、`testing`、`validation` 分类组织；`contract/errors` 承载全局错误码，`contract/pagination` 承载分页契约，`contract/response` 承载 HTTP 响应信封 DTO，`http/binding` 承载 Gin 请求绑定和校验失败响应适配层，`http/response` 承载 Gin 响应输出 helper，`http/middleware` 可承载无业务语义的 Gin 中间件骨架，`security/casbin` 承载无业务语义的 Casbin 三元组和 enforcer 包装，`runtime/rediskey` 承载无业务语义 Redis key 构造规则，`runtime/workerpool` 承载基于 ants 的无业务语义后台任务池 primitive，`testing` 仅承载跨模块测试基础设施和无业务语义 fixture；未来 `runtime/eventbus` 或 `runtime/outbox` 只有在存在跨服务稳定 runtime primitive 和单独设计时才可新增；不得作为服务特定 helper 的兜底目录。
- `user-service/`：用户服务 HTTP 运行时和 Go module，包含 Cobra 入口、Fx 组装、Gin 路由、Ent schema、Atlas migration，以及按 feature 组织的业务代码。
- `user-service/internal/providers/`：用户服务级 Fx provider，集中承载 Gin engine、HTTP route registration、JWT service、PostgreSQL/Redis named resources 和 Ent clients 的服务侧组装；不得承载 feature 业务逻辑。
- `user-service/internal/integration/`：用户服务访问外部系统的防腐层边界，按 `http/`、`grpc/`、`events/` 分类组织；`integration/events` 仅承载外部事件系统 producer/consumer 协议 adapter、envelope/topic 映射和 broker 错误语义归一化，不承载 feature consumer handler、业务编排、outbox persistence 或本服务持久化访问；当前没有真实外部系统调用时只保留 README 或 package doc，占位不得引入未使用代码。
- `user-service/internal/shared/`：用户服务内稳定业务内核边界，只允许已被至少两个 feature 真实消费、边界稳定且不能归入 `common` 的纯类型、值对象、系统内置规格、稳定错误和无副作用判断；当前只开放 `identity` 与 `rbacbaseline`。`identity` 由 user/auth 共同消费，承载用户状态、账号生命周期判断和用户身份错误；`rbacbaseline` 由 role/permission 共同消费，承载系统内置 RBAC 角色、权限和默认角色权限绑定规格。Shared 按稳定业务内核子域建包，不新增根级 `errors`、`enums`、`types`、`utils` 或 `helpers` 兜底包；公共错误放在 owning shared 子包的 `errors.go`，公共枚举按业务语义命名为 `<subject>_status.go`、`<subject>_type.go` 或 `<subject>_kind.go`，系统内置规格放在 owning shared 子包的 `catalog.go` 或更具体的 `<subject>_catalog.go`。新增 shared 子包必须同步在本文件和 `docs/ARCHITECTURE.md` 说明 owner、消费方、准入理由和禁止事项；不得放 Gin、Ent、Redis、SQL、Fx provider、controller、DTO、store port、use case、配置读取、日志副作用、外部调用或 `deployments/` 下的部署资产。
- `user-service/internal/features/user/`：用户资料 feature，按 `application/`、`domain/`、`transport/http/`、`infrastructure/postgres/` 和 `fx.go` 分层；`domain/` 可在有真实纯领域规则或事件模型时按需新增 `services/`、`events/`；`application/command` 承载写侧用例，`application/query` 承载读侧用例，`application/validators` 承载 transport-neutral application 输入辅助；HTTP request/response DTO 位于 `transport/http/request.go`、`response.go`；未来如暴露本服务入站 gRPC API，使用 feature-local `transport/grpc`，当前没有真实 gRPC API 时不得新增业务代码、proto 或 generated code；未来如消费外部事件，使用 feature-local `infrastructure/consumers` 承载入站事件到 application command/query 的 adapter，当前没有真实消费者时不得新增业务代码。
- `user-service/internal/features/auth/`：认证会话 feature，按 `application/`、`domain/`、`transport/http/`、`infrastructure/postgres/`、`infrastructure/redis/` 和 `fx.go` 分层；`domain/services`、`domain/events` 仅在有真实领域服务或领域事件模型时新增；`application/command` 保持扁平并承载登录、刷新、强制改密、退出当前设备和退出全部设备 use case；`application/authctx` 承载认证上下文和客户端审计上下文 helper；`application/credentials` 承载凭据校验和强制改密凭据更新 application 组件；`application/tokens` 承载 JWT 签发解析和 token result DTO；`application/sessions` 承载 refresh session 生命周期、token version fallback、每用户活跃 refresh session 上限策略和会话撤销；`application/validators` 承载 transport-neutral application 输入辅助、token version 撤销校验和 refresh session 一致性校验；`application/ports.go` 继续拥有凭据、token version 和 session ports；`application/query` 只有存在真实 auth 读侧用例时才放实现，当前只可保留 README；HTTP request/response DTO 位于 `transport/http/request.go`、`response.go`；未来如暴露本服务入站 gRPC API，使用 feature-local `transport/grpc`，当前没有真实 gRPC API 时不得新增业务代码、proto 或 generated code；未来如消费外部事件，使用 feature-local `infrastructure/consumers` 承载入站事件到 application command/query 的 adapter，当前没有真实消费者时不得新增业务代码。
- `user-service/internal/features/role/`：角色管理 feature，按 `application/`、`domain/`、`transport/http/`、`infrastructure/postgres/` 和 `fx.go` 分层；`domain/` 承载角色实体、系统角色保护规则和领域错误；`application/command` 承载角色生命周期、用户绑定角色和角色绑定权限写侧用例；`application/query` 承载角色、用户角色和角色权限读侧用例；`application/validators` 承载 transport-neutral 输入辅助；`application/ports.go` 拥有 RoleStore、UserRoleStore、RolePermissionStore 和 PermissionLookup 消费侧端口；HTTP request/response DTO 位于 `transport/http/request.go`、`response.go`；PostgreSQL adapter 使用 Ent 访问 roles、user_roles、role_permissions，并通过 permission feature application 端口校验权限存在且启用，不直接依赖 permission infrastructure。
- `user-service/internal/features/permission/`：权限目录和 RBAC 授权 feature，按 `application/`、`domain/`、`transport/http/`、`infrastructure/postgres/`、`infrastructure/casbin/` 和 `fx.go` 分层；承载权限生命周期、权限查询、有效权限查询、只读路由差异查询、`application/authorization` 授权端口适配、Gin RBAC 授权中间件和 Casbin policy loader/enforcer/reload；系统 RBAC 基线的超级管理员角色、系统权限和默认角色权限绑定由 `internal/shared/rbacbaseline` 拥有，permission seeding、route diff、Casbin adapter 和 role seed 只消费该基线；角色 feature 只能通过 application port 查询权限，不得直接访问 permission infrastructure；Casbin policy subject 使用 `role_id`（`role:<role_uuid>`），不要求 `roles.code` 字段；route diff 只能做已注册路由与正式权限的差异校验，不得创建正式权限或绑定角色。
- `deployments/`：Docker、Compose、Kubernetes 和 Helm 部署资产；`deployments/docker` 放 Dockerfile 或统一构建资产，`deployments/compose` 放本地依赖或本地服务启动配置，`deployments/k8s` 放 Kubernetes YAML，`deployments/helm` 放 Helm chart。

## 3. Key Entry Points

- CLI 入口：`user-service/cmd/main.go`
- 服务组装：`user-service/internal/bootstrap/app.go`
- HTTP server 生命周期：`user-service/internal/bootstrap/server.go`
- 服务级 provider Fx 组装：`user-service/internal/providers/fx.go`
- Gin engine provider：`user-service/internal/providers/gin.go`
- HTTP 路由 provider：`user-service/internal/providers/routes.go`
- 认证 JWT provider：`user-service/internal/providers/auth.go`
- 服务 PostgreSQL provider：`user-service/internal/providers/postgres.go`
- 服务 Redis provider：`user-service/internal/providers/redis.go`
- 服务 Ent provider：`user-service/internal/providers/ent.go`
- Gin router 路由总装：`user-service/internal/router/router.go`
- 健康检查路由：`user-service/internal/router/health.go`
- Swagger 路由：`user-service/internal/router/swagger.go`
- 用户 feature module：`user-service/internal/features/user/fx.go`
- 用户 controller：`user-service/internal/features/user/transport/http/controller.go`
- 用户 HTTP DTO：`user-service/internal/features/user/transport/http/request.go`、`user-service/internal/features/user/transport/http/response.go`
- 用户创建 command service：`user-service/internal/features/user/application/command/create_user.go`
- 用户查询 query service：`user-service/internal/features/user/application/query/query_service.go`
- 用户 PostgreSQL adapter：`user-service/internal/features/user/infrastructure/postgres/user_store.go`
- 认证 feature module：`user-service/internal/features/auth/fx.go`
- 认证 controller：`user-service/internal/features/auth/transport/http/controller.go`
- 认证 HTTP DTO：`user-service/internal/features/auth/transport/http/request.go`、`user-service/internal/features/auth/transport/http/response.go`
- 认证登录 command use case：`user-service/internal/features/auth/application/command/login.go`
- 认证刷新 command use case：`user-service/internal/features/auth/application/command/refresh_token.go`
- 认证改密 command use case：`user-service/internal/features/auth/application/command/change_password.go`
- 认证登出 command use case：`user-service/internal/features/auth/application/command/logout_current_session.go`、`user-service/internal/features/auth/application/command/logout_all_sessions.go`
- 认证上下文 helper：`user-service/internal/features/auth/application/authctx/session.go`、`user-service/internal/features/auth/application/authctx/client_context.go`
- 认证凭据 application 组件：`user-service/internal/features/auth/application/credentials/verifier.go`
- 认证 token application 组件：`user-service/internal/features/auth/application/tokens/issuer.go`、`user-service/internal/features/auth/application/tokens/result.go`
- 认证 session application 组件：`user-service/internal/features/auth/application/sessions/lifecycle.go`、`user-service/internal/features/auth/application/sessions/revocation.go`
- 认证 application validators：`user-service/internal/features/auth/application/validators/auth_validator.go`、`user-service/internal/features/auth/application/validators/token_version_validator.go`、`user-service/internal/features/auth/application/validators/session_policy.go`
- 认证 PostgreSQL adapter：`user-service/internal/features/auth/infrastructure/postgres/credential_store.go`
- 认证 Redis adapter：`user-service/internal/features/auth/infrastructure/redis/session_store.go`
- 角色 feature module：`user-service/internal/features/role/fx.go`
- 角色 controller：`user-service/internal/features/role/transport/http/controller.go`
- 角色 HTTP DTO：`user-service/internal/features/role/transport/http/request.go`、`user-service/internal/features/role/transport/http/response.go`
- 角色写侧 command service：`user-service/internal/features/role/application/command/role.go`、`user-service/internal/features/role/application/command/binding.go`
- 角色读侧 query service：`user-service/internal/features/role/application/query/roles.go`
- 角色 PostgreSQL adapter：`user-service/internal/features/role/infrastructure/postgres/role_store.go`、`user-service/internal/features/role/infrastructure/postgres/user_role_store.go`、`user-service/internal/features/role/infrastructure/postgres/role_permission_store.go`
- 权限 feature module：`user-service/internal/features/permission/fx.go`
- 权限 controller：`user-service/internal/features/permission/transport/http/controller.go`
- 权限 HTTP DTO：`user-service/internal/features/permission/transport/http/request.go`、`user-service/internal/features/permission/transport/http/response.go`
- 权限 RBAC application 授权适配：`user-service/internal/features/permission/application/authorization/authorization.go`
- 用户服务共享身份规格：`user-service/internal/shared/identity/user_status.go`、`user-service/internal/shared/identity/errors.go`
- 用户服务共享 RBAC 基线：`user-service/internal/shared/rbacbaseline/catalog.go`
- 权限 PostgreSQL adapter：`user-service/internal/features/permission/infrastructure/postgres/permission_store.go`
- 权限 Casbin adapter：`user-service/internal/features/permission/infrastructure/casbin/policy.go`、`user-service/internal/features/permission/infrastructure/casbin/enforcer.go`
- 共享 Casbin 授权 helper：`common/security/casbin/authorizer.go`
- 共享 Gin Casbin 授权中间件骨架：`common/http/middleware/casbin.go`
- 共享配置加载：`common/runtime/config/loader.go`
- 共享配置 Fx provider：`common/runtime/config/fx.go`
- 共享日志 Fx provider：`common/runtime/logger/fx.go`
- 共享 datastore Fx provider：`common/runtime/datastore/redis_fx.go`、`common/runtime/datastore/postgres_fx.go`
- 共享 Redis key 构造规则：`common/runtime/rediskey`
- 共享后台任务池：`common/runtime/workerpool`
- 运行时资源名：`common/runtime/resources/resource_names.go`
- 用户服务迁移目录：`user-service/migrations/`，包含 SQL migration、`atlas.sum` 和 Atlas 配置。
- Atlas 配置：`user-service/migrations/atlas.hcl`
- 用户服务 Dockerfile：`deployments/docker/user-service.Dockerfile`

## 4. Current Feature Areas

- 用户资料查询：`GET /api/v1/users/:id`
- 用户资料创建：`POST /api/v1/users`
- 用户列表分页查询：`GET /api/v1/users`
- 认证会话控制：登录、刷新、强制改密、退出当前设备、退出全部设备，以及每用户活跃 refresh session 上限治理。
- 权限目录管理：权限创建、更新、启停、分页查询、详情查询、用户有效权限查询和路由差异查询。
- 角色管理：角色创建、更新、启停、分页查询、详情查询、用户角色绑定查询/替换/增删，以及角色权限绑定查询/替换/增删。
- RBAC HTTP 授权：JWT 认证后对用户、角色、权限业务接口执行 Casbin 授权；Casbin 使用 `user:<user_uuid>`、`role:<role_uuid>`、Gin route template 和 HTTP method，不依赖 `roles.code`。
- RBAC policy 同步：在线 RBAC 管理接口变更权限、角色启停、用户角色绑定或角色权限绑定后，通过本实例 reload、Redis policy version、Pub/Sub 和定时版本补偿刷新其他副本的内存 Casbin policy；授权热路径不做每请求 Redis 强一致版本门控。`rbac seed` 和 `rbac assign-super-admin` 是离线运维入口，应在 migrate 与启动 HTTP server 之间执行；若在已有副本运行中执行，必须滚动重启或另行触发在线 policy refresh。
- HTTP 服务运行时：启动、运行、路由注册和优雅停止。
- 未来入站 gRPC transport 边界：如用户服务暴露真实 gRPC API，放在对应 feature 的 `transport/grpc`；当前不实现 gRPC API、proto、generated code 或 server runtime。
- 外部系统防腐层边界：`internal/integration/http`、`grpc`、`events` 只承载真实外部调用的协议适配规则，当前不实现真实 client；`internal/integration/grpc` 是出站 external client adapter，不是本服务 gRPC server transport；`internal/integration/events` 是外部事件系统 producer/consumer 协议 adapter，不是 feature consumer handler、业务事件编排或 outbox。
- 共享基础设施：配置、日志、Redis/PostgreSQL/Ent 运行时依赖。
- API 响应契约：统一成功/失败响应信封、全局错误码和分页响应模型。
- 数据库迁移：通过 Ent schema 和 Atlas 维护用户服务 SQL migration。

## 5. Development Commands

- 查看统一入口：`make help`。
- 构建用户服务二进制：`make build` 或 `make build-user-service`。
- 运行全部测试：`make test`。
- 运行架构边界检查：`make architecture-lint`。
- 运行完整本地验证：`make verify`。
- 运行用户服务：`make run-user-service`。
- 构建用户服务 Docker 镜像：`docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`。
- 运行单模块测试：`make test-common` 或 `make test-user-service`。
- 生成 Ent 代码：`make generate`。
- 生成迁移：`make migrate-diff name=<name>`。
- 校验迁移：`make migrate-validate`。
- 执行迁移：`DATABASE_URL='<postgres-url>' make migrate-apply`。
- 生成 Swagger 文档：`make swagger-generate`。
- 格式化 Go 代码：`gofmt -w <files>`。
- 运行 lint：`make lint`，或按模块运行 `make lint-common`、`make lint-user-service`。

## 6. Change Workflow

1. 先阅读 `docs/ARCHITECTURE.md`，确认改动属于哪个模块、feature 和层。
2. 小改动直接实现；跨 feature、跨模块、目录结构或外部契约变更，应先在 issue、PR 描述或开发记录中写清目标、影响范围和验证方式。
3. 修改结构规则时，同步更新 `AGENTS.md` 和 `docs/ARCHITECTURE.md`。
4. 实现后运行与改动范围匹配的测试；跨模块变更分别在 `common/` 和 `user-service/` 验证。

## 7. Repository Rules

- 不要手写 `user-service/ent/` 下的生成代码；修改 Ent schema 后重新生成。
- 不要用运行时 `client.Schema.Create(ctx)` 表达 schema 变更；修改 Ent schema 后生成 Ent 代码和 Atlas SQL migration。
- 按 feature 组织服务内代码：用户资料放在 `internal/features/user`，认证会话放在 `internal/features/auth`，角色管理放在 `internal/features/role`，权限目录和 RBAC 授权放在 `internal/features/permission`。不要新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包。
- 保持 `transport/http`、未来 `transport/grpc`、`application`、`domain`、`infrastructure/*` 分层：HTTP controller 固定先用 `binding.BindOrAbort` 完成绑定和结构校验，再通过 feature-local input preparer 完成绑定后的裁剪、默认值归一化、UUID/cursor/token 解析和 command/query 构造；未来 gRPC 解析在 feature-local `transport/grpc` handler，业务编排在 application service 或 application 内的 `command`/`query` 用例，纯领域规则在 domain，数据库或 Redis 访问在 infrastructure adapter。
- `domain/services` 和 `domain/events` 是按需子目录：只有存在真实纯领域服务规则或领域事件模型时才创建；不得为了目录完整新增空 package、空 struct、空 interface 或只含占位注释的业务代码。
- 每个 feature 自己注册路由：`transport/http/routes.go` 暴露 `RegisterRoutes`，认证 feature 可拆分 `RegisterPublicRoutes` 和 `RegisterProtectedRoutes`；全局 router 的 `router.go` 负责 route graph 总装和 `/api/v1` feature 路由分组，`health.go` 负责 `/livez`、`/readyz` 和 `/startupz`，`swagger.go` 负责 Swagger UI 和文档重定向。
- 每个 feature 自己提供 Fx module：`features/<feature>/fx.go` 暴露 `Module` 并组装 feature 内部 service、controller 和 infrastructure provider；服务级 Gin engine、路由注册、JWT、PostgreSQL、Redis 和 Ent provider 放在 `internal/providers`。
- `bootstrap.AppModule` 只负责顶层 Fx module 总装和 HTTP server lifecycle，具体服务级 provider 实现不得放回 `internal/bootstrap`。
- 外部系统防腐层统一使用 `internal/integration`，按 `http/`、`grpc/`、`events/` 分类；不要新增复数 `internal/integrations`。Integration adapter 只做外部协议、DTO、错误语义和 client 调用适配，不承载 feature 业务编排、HTTP controller、gRPC handler、Ent/Redis 持久化 adapter 或预设的 order/payment client。`internal/integration/grpc` 是出站 external client adapter，不是本服务 gRPC server transport。`internal/integration/events` 只承载外部事件系统 broker wrapper、topic/subject/stream 映射、事件 envelope、序列化和 broker 错误语义归一化；事件 producer 的业务决策归 feature application，broker 投递 adapter 归 `integration/events`；事件 consumer 的 broker mechanics 归 `integration/events`，feature-specific 映射和 handler adapter 归对应 feature 的 `infrastructure/consumers`。Feature 内 `domain/events` 只表达领域事实，不等同于 `internal/integration/events` 的外部协议适配。
- Feature 基础设施目录统一使用 `infrastructure/postgres/`、`infrastructure/redis/` 等；未来真实外部事件消费使用 feature-local `infrastructure/consumers/`，只做入站事件到 application command/query 或 port 的 adapter，不承载 broker SDK subscription loop、topic ack/nack 协议、跨 feature orchestration 或直接绕过 application 的业务状态变更；不要使用 `store/` 作为目录名。
- 共享基础能力优先放在 `common/` 对应分类目录中；服务特定规则保留在服务模块内。`common/security/casbin` 只承载 Casbin 请求三元组、通用 enforcer 调用包装、拒绝和未配置错误，不承载 user-service 的 subject schema、角色权限目录、policy loader、super admin baseline 或 route diff。`common/http/middleware` 中的 Casbin 授权中间件只能通过 resolver、authorizer 和 error handler hook 组合服务侧语义，不直接读取服务业务上下文或写入服务特定错误码。`common/runtime/rediskey` 只承载 namespace、分段拼接、prefix 和 Redis Cluster hash tag 等无业务语义 key 构造规则，不承载 auth/user/role/permission 等 feature key schema。Feature 私有缓存、索引、会话、投影和临时 key 由对应 `features/<feature>/infrastructure/redis` 拥有；未来真正通用的 runtime primitive（例如 rate limiter、distributed lock、idempotency）如果进入 `common/runtime/<primitive>`，由该 primitive 自己拥有自己的 key schema。`common/runtime/workerpool` 只承载基于 ants 的受控后台任务执行、并发限制、满载拒绝、日志、统计和 Fx 生命周期管理，不承载 feature 业务规则、业务 DTO、跨 feature 编排、事件投递语义、outbox persistence 或可靠消息语义；长期后台清理不得在 feature adapter 中散落裸 goroutine，应通过该公共 worker pool 或服务侧明确生命周期管理能力提交。高并发正式系统中后台池应按用途命名为专用 Fx 资源，例如 `auth_session_purge_pool`，不得把多个业务场景混用到一个全局共享池。未来 `common/runtime/eventbus` 只有在至少两个服务需要同一稳定、无业务语义 runtime primitive 时才可新增；未来 outbox 必须另开变更设计 transaction boundary、存储模型、投递 worker、重试、幂等和失败策略，设计前不得新增 outbox 表、Ent hook、transaction wrapper 或 dispatcher。
- HTTP API 应使用 `common/contract/response.Envelope` 格式返回，并通过 `common/http/response` 写出 Gin 响应。
- 边界检查不仅看 import 依赖，也要检查人工维护 Go 文件中的函数定义、声明和调用顺序是否服务阅读主线：类型和 Fx 参数结构应位于构造函数或 provider 前；构造函数应位于公开 handler/use case 方法前；HTTP controller handler 顺序应尽量与 `routes.go` 注册顺序一致；私有 helper 可紧跟主要调用方或放在文件尾；发现因顺序导致可读性差、依赖关系混乱或潜在运行错误风险时，应在不改变行为的前提下调整。不得为了排序手写 Ent/Swagger 生成代码。
- 代码注释统一使用中文，函数和方法注释必须使用中文；必要的协议名、库名、HTTP/JWT/Redis/PostgreSQL/Ent/Fx/Gin/trace-id 等技术术语可保留英文。不要为了翻译注释而修改 Go identifier、错误字符串、HTTP 响应消息、配置 key、数据库字段、Redis key 或生成代码。
- Log 日志消息内容必须全部使用英文，日志字段名使用稳定英文 snake_case；HTTP access log 标准字段为 `trace_id`、`user_id`、`client_ip`、`method`、`path`、`status`、`latency_ms`，认证失败安全事件日志额外记录 `user_agent`，但不得记录 password、token、Authorization header、Cookie 或原始请求体；日志级别必须匹配场景严重性，预期业务拒绝不得用 `Error`，系统异常、外部依赖失败、后台任务失败和 panic recover 不得降级为 `Info`。业务日志优先使用 `common/runtime/logger` context helper，避免丢失 trace-id。
- 配置通过 YAML 与 `AEGISCORE_` 环境变量覆盖加载，Redis/PostgreSQL 使用 `redis.<name>` 与 `postgres.<name>` 命名实例，避免硬编码运行时配置。
- `internal/shared` 默认禁止新增。只有当能力已被至少两个 feature 真实消费、边界稳定、且不能归入 `common` 时，才可以新增，并且必须在本文件和 `docs/ARCHITECTURE.md` 说明 owner、消费方、准入理由和禁止事项。Shared 只允许纯类型、值对象、系统内置规格、少量无副作用判断方法和跨 feature 共享的稳定错误；不得导入 feature 包、Gin、Ent、Redis、SQL、Fx、HTTP response、runtime config/logger/datastore provider，不得承载 controller、transport DTO、Swagger DTO、store port、application use case、infrastructure adapter、配置读取、日志副作用、外部系统调用、数据库/缓存访问或部署资产。当前允许的子包只有 `internal/shared/identity` 和 `internal/shared/rbacbaseline`；用户状态与用户身份错误统一使用 `shared/identity`，系统 RBAC 基线统一使用 `shared/rbacbaseline`，不要在 feature 内保留兼容 alias 或重复常量。不得新增根级 `shared/errors`、`shared/enums`、`shared/types`、`shared/utils` 或 `shared/helpers`；公共错误、枚举和值对象必须放入 owning shared 子包，并使用 `errors.go`、`<subject>_status.go`、`<subject>_type.go`、`<subject>_kind.go` 等具体文件名表达语义。
- Ports 由消费侧 feature 拥有：用户资料 command/query 消费的接口放在 `internal/features/user/application/ports.go`，认证 command use case 和 application 组件消费的凭据、token version 和 session 接口放在 `internal/features/auth/application/ports.go`。不要为了 adapter 方便在 infrastructure 包或共享根包定义大接口。影响 refresh token 可续期能力的 session 上限策略由 auth `application/sessions` 持有，并通过 application port 传给 Redis adapter 同步落地；批量物理清理才可使用 `common/runtime/workerpool` 异步执行。
- HTTP request/response DTO、Swagger model、请求 DTO 清洗、绑定后的输入规范化、简单字段解析和 application command/query 构造放在对应 feature 的 `transport/http/request.go`、`response.go`、`input.go`。Controller 的输入处理最多保留两步：`binding.BindOrAbort` 和一个 feature-local preparer；不得在 controller 中串联多个 `NormalizeXXX`、`ParseXXX` helper。Input preparer 不得导入 Ent、Redis、service、infrastructure，不得查询 store、调用 use case、写 HTTP 响应、执行授权或业务存在性判断。
- 未来 gRPC request/response DTO、protobuf 映射、metadata/status 适配和边界 validation 放在对应 feature 的 `transport/grpc`。没有真实 gRPC API 时，只允许 README 或 package doc 占位，不得新增空 handler、空 service、proto、generated code 或 gRPC runtime 依赖。
- Controller 或未来 gRPC handler 必须把 transport DTO 映射为 application command/query 后再调用 service 或 use case，service/use case 不接收 HTTP request/response DTO 或 protobuf DTO。
- RBAC 授权由 permission feature 拥有，系统 RBAC 基线由 `internal/shared/rbacbaseline` 拥有：系统超级管理员角色、系统权限和默认角色权限绑定的唯一长期入口是 `internal/shared/rbacbaseline`；Gin RBAC middleware 位于 `permission/transport/http`，可复用 `common/http/middleware` 的 Casbin 授权骨架，但用户身份解析、路由模板语义和错误响应映射仍由 permission transport 拥有；授权 application wrapper 位于 `permission/application/authorization`，Casbin policy loader/enforcer 位于 `permission/infrastructure/casbin`；loader 只加载启用角色、启用权限和未删除用户的绑定，并基于 `rbacbaseline.SuperAdminRoleID` 补充内置 `super_admin` wildcard policy；在线 RBAC 写路径必须触发 policy refresh 编排，通过 Redis version/Pub/Sub 和版本补偿同步多副本，CLI seed/assign-super-admin 不作为运行期刷新机制。路由差异查询是只读诊断能力，只能返回 missing/stale 差异，不得写权限目录、不得绑定角色。

| 层 | 可以依赖 | 禁止依赖 |
|---|---|---|
| `domain`、`domain/services`、`domain/events` | 标准库、稳定值对象、同 feature domain 模型 | Gin、Ent、Redis、config、logger、response envelope、application ports、infrastructure adapter |
| `application` | `domain`、消费侧端口接口、common 安全原语 | Gin、Ent、Redis、HTTP binder |
| `transport/http` | `application`、Gin、response envelope、feature-local HTTP DTO 和 validation | Ent、Redis、SQL |
| `transport/grpc` | `application`、gRPC/protobuf 边界类型、feature-local gRPC DTO 和 validation | Ent、Redis、SQL、HTTP response envelope、Gin controller、external client adapter |
| `infrastructure/postgres` | Ent、SQL、application ports、domain | Gin、HTTP response |
| `infrastructure/redis` | Redis client、application ports、domain、common runtime primitive | Gin、HTTP response |
| `infrastructure/consumers` | feature application、feature domain、必要的归一化事件输入 DTO | broker SDK subscription loop、Ent/Redis 直接业务访问、Gin、跨 feature orchestration |
| `integration/*` | 外部 SDK/client、feature application ports、domain、common runtime/security 原语 | Gin response、Ent、feature service 业务编排、service-owned persistence adapter |
| `internal/shared/*` | 标准库、稳定无副作用值对象 | feature 包、Gin、Ent、Redis、SQL、Fx、HTTP response、runtime config/logger/datastore provider、controller、DTO、store port、use case |
| `fx.go` | Fx、feature 内部包 | 业务逻辑 |

Adapter 可以做字段裁剪和模型转换，但不得承载复杂业务编排。禁止在 adapter 中实现登录状态机、密码校验、token 签发、跨 store 事务编排或 HTTP 错误映射。

Domain services 可以承载跨实体或跨值对象的纯领域判断，但不得替代 application use case。密码 hash、JWT 签发/解析、Redis session 生命周期、token version cache/database fallback、日志和配置读取仍属于 application、common security、runtime 或 infrastructure 边界；auth 当前将 token/session validation helper 保留在 `application/validators`。Redis key catalog 是 Redis adapter 的存储契约，放在 feature-local `infrastructure/redis` 或 owning runtime primitive 内，不是 domain service 准入样例。Domain events 只承载领域事实的数据模型；事件总线、broker、outbox、publisher、subscriber、consumer handler 或后台投递 worker 必须另开变更设计。

当前没有真实 MQ/broker、eventbus、outbox、producer、subscriber、consumer handler 或后台投递 worker；不得在没有单独设计的情况下新增 Kafka、RabbitMQ、NATS、Redis Stream 等依赖、业务事件发布、outbox 表、Ent hook、transaction wrapper、dispatcher 或 worker。

Ent predicate 构造必须封装在 infrastructure adapter 内，例如 `internal/features/user/infrastructure/postgres/predicates.go`。`application/command`、`application/query` 和 application 根包不得导入 `github.com/aegiscore/user-service/ent/user`，也不得直接调用 Ent predicate。
