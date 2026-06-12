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
- `common/`：跨服务稳定契约和基础能力，按 `contract`、`runtime`、`http`、`security`、`testing`、`validation` 分类组织；`contract/errors` 承载全局错误码，`contract/pagination` 承载分页契约，`contract/response` 承载 HTTP 响应信封 DTO，`http/binding` 承载 Gin 请求绑定和校验失败响应适配层，`http/response` 承载 Gin 响应输出 helper，`runtime/workerpool` 承载基于 ants 的无业务语义后台任务池 primitive，`testing` 仅承载跨模块测试基础设施和无业务语义 fixture；未来 `runtime/eventbus` 或 `runtime/outbox` 只有在存在跨服务稳定 runtime primitive 和单独设计时才可新增；不得作为服务特定 helper 的兜底目录。
- `user-service/`：用户服务 HTTP 运行时和 Go module，包含 Cobra 入口、Fx 组装、Gin 路由、Ent schema、Atlas migration，以及按 feature 组织的业务代码。
- `user-service/internal/providers/`：用户服务级 Fx provider，集中承载 Gin engine、HTTP route registration、JWT service、PostgreSQL/Redis named resources 和 Ent clients 的服务侧组装；不得承载 feature 业务逻辑。
- `user-service/internal/integration/`：用户服务访问外部系统的防腐层边界，按 `http/`、`grpc/`、`events/` 分类组织；`integration/events` 仅承载外部事件系统 producer/consumer 协议 adapter、envelope/topic 映射和 broker 错误语义归一化，不承载 feature consumer handler、业务编排、outbox persistence 或本服务持久化访问；当前没有真实外部系统调用时只保留 README 或 package doc，占位不得引入未使用代码。
- `user-service/internal/features/user/`：用户资料 feature，按 `application/`、`domain/`、`transport/http/`、`infrastructure/postgres/` 和 `fx.go` 分层；`domain/` 可在有真实纯领域规则或事件模型时按需新增 `services/`、`events/`；`application/command` 承载写侧用例，`application/query` 承载读侧用例，`application/validators` 承载 transport-neutral application 输入辅助；HTTP request/response DTO 位于 `transport/http/request.go`、`response.go`；未来如暴露本服务入站 gRPC API，使用 feature-local `transport/grpc`，当前没有真实 gRPC API 时不得新增业务代码、proto 或 generated code；未来如消费外部事件，使用 feature-local `infrastructure/consumers` 承载入站事件到 application command/query 的 adapter，当前没有真实消费者时不得新增业务代码。
- `user-service/internal/features/auth/`：认证会话 feature，按 `application/`、`domain/`、`transport/http/`、`infrastructure/postgres/`、`infrastructure/redis/` 和 `fx.go` 分层；`domain/services`、`domain/events` 仅在有真实领域服务或领域事件模型时新增；`application/command` 承载登录、刷新、强制改密、退出当前设备和退出全部设备 use case，`application/validators` 承载 transport-neutral application 输入辅助、token version 撤销校验、cache/database fallback 策略和 refresh session 一致性校验，`application/ports.go` 继续拥有凭据、token version 和 session ports；HTTP request/response DTO 位于 `transport/http/request.go`、`response.go`；未来如暴露本服务入站 gRPC API，使用 feature-local `transport/grpc`，当前没有真实 gRPC API 时不得新增业务代码、proto 或 generated code；未来如消费外部事件，使用 feature-local `infrastructure/consumers` 承载入站事件到 application command/query 的 adapter，当前没有真实消费者时不得新增业务代码。
- `user-service/internal/features/role/`、`user-service/internal/features/permission/`：未来 RBAC 能力的最小 feature skeleton，仅保留 README 标注边界；当前不注册路由、不提供 Fx module、不新增 application/domain/infrastructure 代码、不新增 Ent schema 或数据库表。
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
- 认证刷新 command use case：`user-service/internal/features/auth/application/command/refresh.go`
- 认证改密 command use case：`user-service/internal/features/auth/application/command/change_password.go`
- 认证登出 command use case：`user-service/internal/features/auth/application/command/logout_current_session.go`、`user-service/internal/features/auth/application/command/logout_all_sessions.go`
- 认证 application validators：`user-service/internal/features/auth/application/validators/auth_validator.go`、`user-service/internal/features/auth/application/validators/token_version_validator.go`、`user-service/internal/features/auth/application/validators/session_policy.go`
- 认证 PostgreSQL adapter：`user-service/internal/features/auth/infrastructure/postgres/credential_store.go`
- 认证 Redis adapter：`user-service/internal/features/auth/infrastructure/redis/session_store.go`
- 共享配置加载：`common/runtime/config/loader.go`
- 共享配置 Fx provider：`common/runtime/config/fx.go`
- 共享日志 Fx provider：`common/runtime/logger/fx.go`
- 共享 datastore Fx provider：`common/runtime/datastore/redis_fx.go`、`common/runtime/datastore/postgres_fx.go`
- 共享后台任务池：`common/runtime/workerpool`
- 运行时资源名：`common/runtime/resources/resource_names.go`
- 用户服务迁移目录：`user-service/migrations/`，包含 SQL migration、`atlas.sum` 和 Atlas 配置。
- Atlas 配置：`user-service/migrations/atlas.hcl`
- 用户服务 Dockerfile：`deployments/docker/user-service.Dockerfile`

## 4. Current Feature Areas

- 用户资料查询：`GET /api/v1/users/:id`
- 用户资料创建：`POST /api/v1/users`
- 用户列表分页查询：`GET /api/v1/users`
- 认证会话控制：登录、刷新、强制改密、退出当前设备、退出全部设备。
- RBAC future feature skeleton：`internal/features/role` 和 `internal/features/permission` 只标注未来角色权限边界，当前不实现角色权限业务。
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
- 按 feature 组织服务内代码：用户资料放在 `internal/features/user`，认证会话放在 `internal/features/auth`。不要新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包。
- 保持 `transport/http`、未来 `transport/grpc`、`application`、`domain`、`infrastructure/*` 分层：HTTP 解析在 controller，未来 gRPC 解析在 feature-local `transport/grpc` handler，业务编排在 application service 或 application 内的 `command`/`query` 用例，纯领域规则在 domain，数据库或 Redis 访问在 infrastructure adapter。
- `domain/services` 和 `domain/events` 是按需子目录：只有存在真实纯领域服务规则或领域事件模型时才创建；不得为了目录完整新增空 package、空 struct、空 interface 或只含占位注释的业务代码。
- 每个 feature 自己注册路由：`transport/http/routes.go` 暴露 `RegisterRoutes`，认证 feature 可拆分 `RegisterPublicRoutes` 和 `RegisterProtectedRoutes`；全局 router 的 `router.go` 负责 route graph 总装和 `/api/v1` feature 路由分组，`health.go` 负责 `/healthz`，`swagger.go` 负责 Swagger UI 和文档重定向。
- 每个 feature 自己提供 Fx module：`features/<feature>/fx.go` 暴露 `Module` 并组装 feature 内部 service、controller 和 infrastructure provider；服务级 Gin engine、路由注册、JWT、PostgreSQL、Redis 和 Ent provider 放在 `internal/providers`。
- `bootstrap.AppModule` 只负责顶层 Fx module 总装和 HTTP server lifecycle，具体服务级 provider 实现不得放回 `internal/bootstrap`。
- 外部系统防腐层统一使用 `internal/integration`，按 `http/`、`grpc/`、`events/` 分类；不要新增复数 `internal/integrations`。Integration adapter 只做外部协议、DTO、错误语义和 client 调用适配，不承载 feature 业务编排、HTTP controller、gRPC handler、Ent/Redis 持久化 adapter 或预设的 order/payment client。`internal/integration/grpc` 是出站 external client adapter，不是本服务 gRPC server transport。`internal/integration/events` 只承载外部事件系统 broker wrapper、topic/subject/stream 映射、事件 envelope、序列化和 broker 错误语义归一化；事件 producer 的业务决策归 feature application，broker 投递 adapter 归 `integration/events`；事件 consumer 的 broker mechanics 归 `integration/events`，feature-specific 映射和 handler adapter 归对应 feature 的 `infrastructure/consumers`。Feature 内 `domain/events` 只表达领域事实，不等同于 `internal/integration/events` 的外部协议适配。
- Feature 基础设施目录统一使用 `infrastructure/postgres/`、`infrastructure/redis/` 等；未来真实外部事件消费使用 feature-local `infrastructure/consumers/`，只做入站事件到 application command/query 或 port 的 adapter，不承载 broker SDK subscription loop、topic ack/nack 协议、跨 feature orchestration 或直接绕过 application 的业务状态变更；不要使用 `store/` 作为目录名。
- 共享基础能力优先放在 `common/` 对应分类目录中；服务特定规则保留在服务模块内。`common/runtime/workerpool` 只承载基于 ants 的受控后台任务执行、并发限制、满载拒绝、日志、统计和 Fx 生命周期管理，不承载 feature 业务规则、业务 DTO、跨 feature 编排、事件投递语义、outbox persistence 或可靠消息语义；长期后台清理不得在 feature adapter 中散落裸 goroutine，应通过该公共 worker pool 或服务侧明确生命周期管理能力提交。高并发正式系统中后台池应按用途命名为专用 Fx 资源，例如 `auth_session_purge_pool`，不得把多个业务场景混用到一个全局共享池。未来 `common/runtime/eventbus` 只有在至少两个服务需要同一稳定、无业务语义 runtime primitive 时才可新增；未来 outbox 必须另开变更设计 transaction boundary、存储模型、投递 worker、重试、幂等和失败策略，设计前不得新增 outbox 表、Ent hook、transaction wrapper 或 dispatcher。
- HTTP API 应使用 `common/contract/response.Envelope` 格式返回，并通过 `common/http/response` 写出 Gin 响应。
- 代码注释统一使用中文，函数和方法注释必须使用中文；必要的协议名、库名、HTTP/JWT/Redis/PostgreSQL/Ent/Fx/Gin/trace-id 等技术术语可保留英文。不要为了翻译注释而修改 Go identifier、错误字符串、HTTP 响应消息、配置 key、数据库字段、Redis key 或生成代码。
- Log 日志消息内容必须全部使用英文，日志字段名使用稳定英文 snake_case；HTTP access log 标准字段为 `trace_id`、`user_id`、`client_ip`、`method`、`path`、`status`、`latency_ms`，认证失败安全事件日志额外记录 `user_agent`，但不得记录 password、token、Authorization header、Cookie 或原始请求体；日志级别必须匹配场景严重性，预期业务拒绝不得用 `Error`，系统异常、外部依赖失败、后台任务失败和 panic recover 不得降级为 `Info`。业务日志优先使用 `common/runtime/logger` context helper，避免丢失 trace-id。
- 配置通过 YAML 与 `AEGISCORE_` 环境变量覆盖加载，Redis/PostgreSQL 使用 `redis.<name>` 与 `postgres.<name>` 命名实例，避免硬编码运行时配置。
- `internal/shared` 默认禁止新增。只有当能力已被至少两个 feature 真实消费、边界稳定、且不能归入 `common` 时，才可以新增，并且必须在 `docs/ARCHITECTURE.md` 说明 owner、准入理由和禁止事项。
- Ports 由消费侧 feature 拥有：用户资料 command/query 消费的接口放在 `internal/features/user/application/ports.go`，认证 command use case 消费的凭据、token version 和 session 接口放在 `internal/features/auth/application/ports.go`。不要为了 adapter 方便在 infrastructure 包或共享根包定义大接口。
- HTTP request/response DTO、Swagger model、请求 DTO 清洗、绑定后的输入规范化和简单字段解析放在对应 feature 的 `transport/http/request.go`、`response.go`、`validation.go`。这些函数不得导入 Ent、Redis、service、infrastructure，或执行业务编排。
- 未来 gRPC request/response DTO、protobuf 映射、metadata/status 适配和边界 validation 放在对应 feature 的 `transport/grpc`。没有真实 gRPC API 时，只允许 README 或 package doc 占位，不得新增空 handler、空 service、proto、generated code 或 gRPC runtime 依赖。
- Controller 或未来 gRPC handler 必须把 transport DTO 映射为 application command/query 后再调用 service 或 use case，service/use case 不接收 HTTP request/response DTO 或 protobuf DTO。

| 层 | 可以依赖 | 禁止依赖 |
|---|---|---|
| `domain`、`domain/services`、`domain/events` | 标准库、稳定值对象、同 feature domain 模型 | Gin、Ent、Redis、config、logger、response envelope、application ports、infrastructure adapter |
| `application` | `domain`、消费侧端口接口、common 安全原语 | Gin、Ent、Redis、HTTP binder |
| `transport/http` | `application`、Gin、response envelope、feature-local HTTP DTO 和 validation | Ent、Redis、SQL |
| `transport/grpc` | `application`、gRPC/protobuf 边界类型、feature-local gRPC DTO 和 validation | Ent、Redis、SQL、HTTP response envelope、Gin controller、external client adapter |
| `infrastructure/postgres` | Ent、SQL、application ports、domain | Gin、HTTP response |
| `infrastructure/redis` | Redis client、application ports、domain | Gin、HTTP response |
| `infrastructure/consumers` | feature application、feature domain、必要的归一化事件输入 DTO | broker SDK subscription loop、Ent/Redis 直接业务访问、Gin、跨 feature orchestration |
| `integration/*` | 外部 SDK/client、feature application ports、domain、common runtime/security 原语 | Gin response、Ent、feature service 业务编排、service-owned persistence adapter |
| `fx.go` | Fx、feature 内部包 | 业务逻辑 |

Adapter 可以做字段裁剪和模型转换，但不得承载复杂业务编排。禁止在 adapter 中实现登录状态机、密码校验、token 签发、跨 store 事务编排或 HTTP 错误映射。

Domain services 可以承载跨实体或跨值对象的纯领域判断，但不得替代 application use case。密码 hash、JWT 签发/解析、Redis session 生命周期、token version cache/database fallback、日志和配置读取仍属于 application、common security、runtime 或 infrastructure 边界；auth 当前将 token/session validation helper 保留在 `application/validators`。Auth 当前的 `RedisKeyBuilder` 依赖 runtime config 并服务 Redis key 构造，不是 domain service 准入样例。Domain events 只承载领域事实的数据模型；事件总线、broker、outbox、publisher、subscriber、consumer handler 或后台投递 worker 必须另开变更设计。

当前没有真实 MQ/broker、eventbus、outbox、producer、subscriber、consumer handler 或后台投递 worker；不得在没有单独设计的情况下新增 Kafka、RabbitMQ、NATS、Redis Stream 等依赖、业务事件发布、outbox 表、Ent hook、transaction wrapper、dispatcher 或 worker。

Ent predicate 构造必须封装在 infrastructure adapter 内，例如 `internal/features/user/infrastructure/postgres/predicates.go`。`application/command`、`application/query` 和 application 根包不得导入 `github.com/aegiscore/user-service/ent/user`，也不得直接调用 Ent predicate。
