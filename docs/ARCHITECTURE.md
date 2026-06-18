# Architecture

## 1. Overview

AegisCore 是 Go 1.26 workspace，当前包含共享基础模块 `common` 和用户服务模块 `user-service`。用户服务通过 Cobra 提供 CLI，通过 Uber Fx 组装依赖，通过 Gin 暴露 HTTP API，通过 Ent 访问 PostgreSQL，并通过 Atlas 维护服务内 SQL migration。

本文件和根目录 `AGENTS.md` 是仓库结构与分层规则的唯一长期规则源。仓库不再维护 OpenSpec/OPSX 工件。

## 2. Module Boundaries

| 模块 | 责任 | 关键位置 |
|---|---|---|
| `common` | 跨服务稳定契约与基础能力；不得承载服务特定 helper 或业务语义 | `common/contract/`, `common/runtime/`, `common/http/`, `common/security/`, `common/testing/`, `common/validation/` |
| `user-service` | 用户服务运行时、用户服务内共享业务内核、用户资料、认证会话、角色管理、权限目录与 RBAC 授权 feature、外部系统防腐层边界、Ent schema、Atlas migration、服务侧 OpenAPI 生成脚本和 OpenAPI 3 文档 | `user-service/cmd/`, `user-service/internal/`, `user-service/internal/shared/`, `user-service/internal/integration/`, `user-service/ent/`, `user-service/migrations/`, `user-service/docs/` |
| `deployments` | 本地和生产部署资产；Docker build、Compose、本地依赖、Kubernetes YAML、Helm chart 和观测 dashboard/alert 资产的归属边界 | `deployments/docker/user-service.Dockerfile`, `deployments/compose/`, `deployments/k8s/`, `deployments/helm/`, `deployments/observability/` |

仓库根目录是 workspace，不是业务 Go module。运行 Go 命令时通常进入 `common/` 或 `user-service/`。

## 3. Runtime Flow

1. `user-service/cmd/main.go` 创建 `aegiscore-user-services` CLI，并注册 `serve` 子命令。
2. `serve` 调用 `bootstrap.NewApp(configPath)` 创建 Fx 应用。
3. `user-service/internal/bootstrap.AppModule` 导入共享 runtime module、feature modules、`providers.Module`，并提供 HTTP server 生命周期。
4. `user-service/internal/providers.Module` 显式提供 Redis/PostgreSQL named providers、Ent clients、JWT service、metrics provider、Gin engine 和 HTTP route registration。
5. User/Auth/Role/Permission feature modules 自己组装 feature-local infrastructure adapter、application service 或 command/query use case、授权组件和 HTTP controller。
6. `user-service/internal/providers/routes.go` 适配依赖并调用 `router.RegisterUserServiceHTTPRoutes`；`router.go` 负责 route graph 总装和 `/api/v1` 分组，`health.go` 注册 `/livez`、`/readyz`、`/startupz` 探针，`metrics.go` 在启用 metrics 时注册配置化 Prometheus scrape endpoint，`openapi.go` 注册 OpenAPI UI、OpenAPI JSON 和文档重定向。
7. Fx lifecycle 启动 HTTP server，并在进程收到中断或 SIGTERM 时优雅关闭。

`aegiscore-user-services` 是当前运行时 CLI/service name，不是仓库目录名或 Go module path；代码位置和 module path 统一使用 `user-service`。

### Shutdown Order

用户服务关闭必须显式保持以下顺序：

1. 先停止 HTTP server 接收新请求。
2. 再等待已经进入 handler 的请求处理完成。
3. 最后关闭 Ent client、PostgreSQL/Redis 连接和后台任务池等底层资源。

当前实现由 `user-service/internal/bootstrap/server.go` 和 Fx 生命周期注册顺序共同保证。`NewHTTPServer` 的 `OnStop` 调用 `http.Server.Shutdown`；该调用会先关闭 listener 和 idle connection，使服务不再接收新请求，然后等待 active connection 归零。HTTP handler 外层还包裹 `httpDrainTracker`：如果 `http.shutdown_timeout` 到期导致 graceful shutdown 返回错误，`OnStop` 会调用 `server.Close()` 关闭活跃连接，并继续等待已经进入 handler 的请求退出，直到它们完成或外层 Fx stop context 到期。这样在 stop context 仍有预算时，不会在 handler 仍运行时继续关闭数据库连接。

Fx 的 `OnStop` hook 按成功 `OnStart` hook 的反向注册顺序执行。`AppModule` 中 `providers.Module` 位于 `NewHTTPServer` provider 和 `fx.Invoke(func(*http.Server){})` 之前；HTTP server 被 invoke 实例化前，PostgreSQL、Redis、Ent client、auth session purge worker pool 等依赖已经因 feature/controller/router 依赖链完成构造并注册各自 hook。因此当前关闭顺序是：HTTP server `OnStop` 先执行并 drain 请求，然后 auth session purge worker pool 停止，随后 Ent client 关闭，最后 PostgreSQL/Redis 等 datastore 连接关闭。Ent client 使用 non-closing driver 包装具名 `*sql.DB`，自身关闭不负责关闭底层连接池；真正的 SQL pool 关闭仍由 datastore provider 的 hook 执行。

如果 HTTP handler 忽略请求 context 且在 `server.Close()` 后仍不退出，`httpDrainTracker` 会一直等到外层 Fx stop context 到期。此时 Fx stop 会因 context 超时返回，后续 datastore hook 不应被当作已安全执行；调用方需要把这类超时视为 graceful shutdown 失败并排查阻塞 handler。

## 4. HTTP Request Flow

| 步骤 | 代码位置 | 行为 |
|---|---|---|
| 中间件链 | `user-service/internal/providers/gin.go` | 创建 Gin engine，注册 OTel Gin tracing、HTTP server RED metrics、panic recovery、request logging、CORS |
| 路由 provider | `user-service/internal/providers/routes.go` | 将 Fx 依赖适配为 router route params |
| 路由总装 | `user-service/internal/router/router.go`、`health.go`、`metrics.go`、`openapi.go` | `router.go` 创建 public/protected route groups 并总装 route graph，`health.go` 注册 `/livez`、`/readyz`、`/startupz` 探针，`metrics.go` 在启用 metrics 时注册配置化 Prometheus scrape endpoint，`openapi.go` 注册 OpenAPI UI、OpenAPI JSON 和文档重定向 |
| 参数解析 | `features/*/transport/http/controller.go`、`features/*/transport/http/input.go` | Controller 使用 `binding.BindOrAbort` 绑定 HTTP DTO 并执行结构校验；feature-local input preparer 负责绑定后的裁剪、默认值归一化、UUID/cursor/token 解析，并映射为 command/query |
| 业务调用 | `features/*/application/` | 编排用户资料、认证会话、角色管理、权限目录和 RBAC 授权用例；用户资料 feature 的读写用例分别位于 `application/query` 与 `application/command`，认证会话 feature 的登录、刷新、强制改密和登出用例位于 `application/command`，角色 feature 使用 command/query 管理角色与绑定，permission feature 使用 command/query 管理权限目录、有效权限、route diff 和 authorization wrapper，并复用 `authctx`、`credentials`、`tokens`、`sessions` application 组件 |
| 数据访问 | `features/*/infrastructure/postgres/`, `features/*/infrastructure/redis/` | 使用 Ent 或 Redis 访问持久化细节，转换存储层错误 |
| 响应输出 | `common/http/response/` + `common/contract/response/` | 通过 Gin writer 输出统一 `success/code/message/data` 信封，并复用稳定错误码与分页契约 |

## 5. Feature-First Organization

服务内业务代码按 feature 组织在 `user-service/internal/features/<feature>/`。当前稳定 feature 包括：

- `user`：用户资料创建、查询和分页列表。
- `auth`：登录、刷新、强制改密、退出当前设备、退出全部设备。
- `permission`：权限目录生命周期、分页查询、用户有效权限查询、只读已注册 HTTP 路由差异查询和 RBAC 授权；系统 RBAC 基线由 `internal/shared/rbacbaseline` 统一拥有，permission 只消费该基线，`application/authorization` 负责授权端口适配，`transport/http` 拥有 Gin RBAC middleware，`infrastructure/casbin` 拥有 Casbin policy loader/enforcer/reload。
- `role`：角色生命周期、用户角色绑定、角色权限绑定和角色查询；角色绑定权限前通过 permission feature application 端口校验权限存在且启用，不直接依赖 permission infrastructure。

每个 feature 使用以下分层：

| 目录 | 责任 |
|---|---|
| `application/` | service、commands、queries、ports、use case mapper 和业务编排；可按 feature 需要细分为 `command/`、`query/`、`validators/` 和稳定组件包。Auth 当前使用扁平 `command/` 承载登录、刷新、强制改密和登出 use case；使用 `authctx/` 承载认证上下文 helper；使用 `credentials/` 承载凭据校验和强制改密凭据更新；使用 `tokens/` 承载 JWT 签发解析和 token result DTO；使用 `sessions/` 承载 refresh session 生命周期、token version fallback、每用户活跃 refresh session 上限策略和会话撤销；使用 `validators/` 承载 transport-neutral 输入辅助、token version 撤销校验和 refresh session 一致性校验。Auth 当前没有真实读侧 query，`application/query` 只保留 README 边界说明 |
| `domain/` | 领域实体、值对象、枚举、领域错误和纯业务规则；可按真实需要细分 `services/` 承载跨实体或跨值对象的纯领域服务规则，`events/` 承载纯领域事件模型 |
| `transport/http/` | 当前已实现的入站 HTTP transport，承载 Gin controller、route registration、HTTP request/response DTO、OpenAPI 文档模型、HTTP DTO validation、feature-local input preparer 和边界映射 |
| `transport/grpc/` | 未来本服务暴露入站 gRPC API 时的 feature-local transport，承载 gRPC handler、server-side protobuf request/response 映射、gRPC 边界 validation 和 application command/query 映射；当前没有真实 gRPC API 时不得新增业务代码、空 handler、空 service、未使用 proto 或 generated code，只可按需保留 README 或 package doc |
| `infrastructure/postgres/` | Ent/PostgreSQL adapter 和 predicate 构造 |
| `infrastructure/redis/` | Redis adapter；仅在 feature 需要 Redis 时存在 |
| `infrastructure/consumers/` | 未来 feature-local 入站事件 consumer adapter；仅当该 feature 有真实外部事件消费需求时新增，负责把已归一化的事件输入映射为 application command/query 或 port 调用 |
| `fx.go` | Feature-local Fx module，组装 application、transport 和 infrastructure provider |

Feature transport 可以按入站协议拆分在同一 feature 的 `transport/` 下。`transport/http` 和未来 `transport/grpc` 都必须把协议 DTO 映射为 transport-neutral application command/query 后再调用用例，不得互相导入对方 controller、DTO 或 route。HTTP controller 的输入处理应保持两步式：第一步固定调用 `binding.BindOrAbort` 完成绑定和结构校验，第二步调用 feature-local input preparer 完成绑定后的裁剪、默认值归一化、UUID/cursor/token 解析和 command/query 构造；controller 不直接串联多个 `NormalizeXXX`、`ParseXXX` helper。可复用的输入辅助优先沉淀到 application `validators/`、`authctx/`、domain 值对象或其他 transport-neutral 层，而不是在 HTTP 和 gRPC transport 之间横向复用协议 DTO。

`domain/services` 和 `domain/events` 是按需子目录，只在存在真实纯领域服务规则或领域事件模型时创建；不要为了目录完整新增空 package、空 struct、空 interface 或只含占位注释的业务代码。简单实体、值对象、枚举、领域错误和单实体短方法仍可留在 `domain/` 根部。Auth 当前将 token/session validation helper 保留在 `application/validators`；当前没有领域事件模型，也没有事件总线实现。

### RBAC Authorization Boundary

RBAC 授权由 permission feature 拥有，系统 RBAC 基线由 `internal/shared/rbacbaseline` 拥有。`internal/shared/rbacbaseline` 是系统超级管理员角色、系统权限和默认角色权限绑定的唯一长期入口；role seed、permission route diff 和 Casbin adapter 只消费该基线，不再各自维护重复常量。HTTP route graph 中，JWT 认证先写入用户上下文，Gin RBAC middleware 再对用户、角色、权限等业务接口执行授权；健康检查、OpenAPI 文档、公有 auth 路由、已认证但不做 RBAC 的 auth session 路由和 `OPTIONS` 请求不进入 RBAC 授权。Casbin object 使用 Gin `c.FullPath()` 得到的 route template，action 使用 HTTP method，subject 使用 `user:<user_uuid>` 和 `role:<role_uuid>`；policy loader 使用角色 UUID 作为 `role_id` 主体，不要求 `roles.code` 字段。

Casbin policy loader 只加载未删除用户、启用角色、启用权限以及仍存在的用户角色和角色权限绑定，并基于 `rbacbaseline.SuperAdminRoleID` 补充内置 `super_admin` wildcard policy。角色 feature 绑定权限前只通过 permission application port 校验权限存在且启用，不直接依赖 permission infrastructure 或 Casbin 包。Permission route diff 是只读诊断能力，只比较 Gin 已注册可授权路由与正式权限目录的 missing/stale 差异，不创建权限、不修改权限状态、不绑定角色。

在线 RBAC 管理接口完成权限、角色状态、用户角色绑定或角色权限绑定变更后，必须触发 permission feature 的 policy refresh 编排：本实例先全量 reload 内存 Casbin policy，再通过 Redis policy version 与 Pub/Sub 通知其他副本；其他副本收到通知后 reload，并通过定时版本补偿检查覆盖漏收消息场景。因此 HTTP 授权链路不在每次请求前读取 Redis 做强一致版本门控，运行时策略同步语义是短暂最终一致。`rbac seed` 和 `rbac assign-super-admin` 是离线运维入口，不由 HTTP `serve` 自动执行，也不承担运行期跨副本刷新；生产环境应在 migrate 与启动 HTTP server 之间执行，若必须在已有副本运行中执行，则需安排滚动重启或另行触发在线 policy refresh。

不要新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包。跨 feature 的共享业务代码也不要默认放到 `internal/shared`；只有当能力已被至少两个 feature 真实消费、边界稳定、且不能归入 `common` 时，才可以新增，并需在本文件补充 owner、消费方、准入理由和禁止事项。

### User Service Shared Kernel

`user-service/internal/shared` 是用户服务内稳定业务内核边界，不是跨服务公共库，也不是 feature 外的 helper 兜底目录。进入 shared 的能力必须同时满足：已被至少两个用户服务 feature 真实消费；表达稳定业务规格、纯类型、值对象、系统内置规格、稳定错误或少量无副作用判断方法；不能归入跨服务无业务语义的 `common`；新增子包时同步更新 `AGENTS.md` 和本文档说明 owner、消费方、准入理由和禁止事项。

`internal/shared` 禁止导入或承载 Gin、Ent、Redis、SQL、Fx provider、controller、transport DTO、OpenAPI 文档 DTO、store port、application use case、feature infrastructure adapter、feature application service、HTTP response helper、配置读取、日志副作用、外部系统调用、数据库访问或缓存访问。

Shared 按稳定业务内核子域建包，不按 feature 名称或技术类型建兜底目录。不得新增根级 `shared/errors`、`shared/enums`、`shared/types`、`shared/utils` 或 `shared/helpers`。公共错误放在 owning shared 子包的 `errors.go`；公共枚举按业务语义命名为 `<subject>_status.go`、`<subject>_type.go` 或 `<subject>_kind.go`；系统内置规格或目录数据放在 owning shared 子包的 `catalog.go` 或更具体的 `<subject>_catalog.go`。`deployments/` 下的 Docker、Compose、Kubernetes 和 Helm 资产不属于 shared，不能迁入 `internal/shared`。

当前允许的 shared 子包只有：

- `internal/shared/identity`：user/auth 共同业务内核，owner 是 user 与 auth。该包承载用户状态、账号生命周期判断和用户身份错误；用户状态文件命名为 `user_status.go`，用户身份错误保留在 `errors.go`。用户资料创建、查询、认证登录、强制改密、凭据 adapter、角色用户绑定查询等需要用户状态或用户身份错误时，统一消费该包，不在 feature domain 内保留兼容 alias 或重复错误。
- `internal/shared/rbacbaseline`：role/permission 共同 RBAC 系统规格，owner 是 role 与 permission。该包承载系统超级管理员角色、系统权限和默认角色权限绑定规格。Permission 继续拥有权限目录生命周期、有效权限查询、只读 route diff、authorization wrapper、Gin RBAC middleware 和 Casbin adapter；Role 继续拥有角色生命周期和角色绑定 use case；shared baseline 只表达系统初始规格，不做目录写入、不加载 policy、不访问 store。

服务级 provider 统一放在 `user-service/internal/providers`。该包只负责把共享 runtime、common security、Gin、router 和 Ent 适配为用户服务 Fx 依赖；不得承载 feature 业务逻辑、HTTP route 定义或跨服务共享基础能力。`internal/bootstrap` 只负责 `fx.New`、顶层 `AppModule` 总装和 HTTP server 生命周期。

外部系统防腐层统一放在 `user-service/internal/integration`，并按 `http/`、`grpc/`、`events/` 分类。该边界只在有真实外部系统调用时承载协议 client adapter、外部 DTO 映射、外部错误语义归一化和传输细节；当前没有真实外部调用时只保留 README 或 package doc。`internal/integration/grpc` 只表示用户服务调用外部 gRPC service 的出站 client adapter，不承载本服务 gRPC server、handler、route 或 feature transport 逻辑。`internal/integration/events` 只表示用户服务访问外部事件系统的 producer/consumer 协议 adapter 边界，承载 broker wrapper、topic/subject/stream 映射、事件 envelope、序列化和 broker 错误语义归一化；它不承载 feature-specific consumer handler、application command/query 实现、业务编排、本服务持久化访问或 outbox persistence。Feature 内 `domain/events` 只表达领域事实，不等同于 `internal/integration/events` 的外部协议适配。`integration` 不属于 feature 内部业务编排，不拥有用例流程、登录状态机、跨 store 事务、HTTP controller、gRPC handler 或本服务持久化访问。Feature application service 或 command/query 用例仍通过消费侧 ports 表达外部能力需求，integration adapter 只实现这些最小接口。

未来事件 producer 路径应是 feature application use case 决定业务意图，通过 feature-owned publish port 交给 `integration/events` producer adapter，再由 adapter 调用真实外部 broker。未来事件 consumer 路径应是外部 broker 先进入 `integration/events` consumer wrapper，转换为归一化入站事件输入后，再交给 `features/<feature>/infrastructure/consumers` 的 feature-local adapter 映射到 application command/query 或 port。Feature `infrastructure/consumers` 不承载 broker SDK subscription loop、topic ack/nack 协议、跨 feature orchestration 或直接绕过 application 的业务状态变更。

## 6. Dependency Rules

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

Ports 由消费侧 feature 拥有。Infrastructure adapter 只实现 application 层定义的最小接口，不为了自身方便定义大接口。Auth 的每用户活跃 refresh session 上限属于 application 持有的安全策略，通过 application port 传入 Redis adapter 同步执行；该策略不得下沉为 Redis adapter 私有配置，也不得交给后台 worker 异步补偿。

Controller 或未来 gRPC handler 必须把 transport DTO 映射为 application command/query 后再调用 service 或用例。HTTP controller 应通过 feature-local input preparer 完成 HTTP DTO 到 application command/query 的输入准备，preparer 不查询 store、不调用 use case、不写响应、不承载授权或业务存在性判断。Service 和 command/query 用例不接收 HTTP request/response DTO 或 protobuf DTO，也不导入 Gin、Ent predicate、Redis client、HTTP binder 或 gRPC runtime。

Ent predicate 构造封装在 `infrastructure/postgres` 内。Adapter 可以做字段裁剪、模型转换和存储错误转换，但不得承载复杂业务编排、登录状态机、密码校验、token 签发、跨 store 事务编排或 HTTP 错误映射。

边界检查不仅覆盖 import 依赖，也覆盖人工维护 Go 文件中的函数定义、声明和调用顺序。检查时应确认 const、var、sentinel error、类型、接口、Fx 参数结构和输出结构位于依赖它们的构造函数或 provider 前；构造函数和 provider 位于公开 handler、service 或 use case 方法前；HTTP controller 的 handler 顺序尽量与同包 `routes.go` 注册顺序一致；私有 helper 可以紧跟主要调用方，也可以在文件尾按调用链组织。无关的 type、const 或 var 不应穿插在两个主要函数之间打断阅读主线。测试文件可以按场景组织 helper，但 helper 不应打断主要测试用例的阅读主线。若顺序导致可读性差、依赖关系混乱或潜在运行错误风险，应在不改变功能的前提下整理。Ent、OpenAPI 等生成代码不为顺序检查手写调整，必须通过对应生成流程更新。

正式代码不得暴露仅为测试服务的构造函数、方法、hook、flag 或可替换入口，例如 `NewXForTest`、`testHook`、`setNowForTest` 这类测试语义 API。确实需要可替换性时，必须有清晰运行时职责并使用运行时语义命名，例如消费侧端口、Fx provider 参数、clock/ID generator 等稳定运行时依赖；否则测试替身、fake、stub、fixture、时间控制和特殊断言入口应留在 `_test.go`、`common/testing` 或对应测试基础设施中。不得为了让测试方便而把临时分支、测试专用参数或无运行时职责的 helper 留在正式构建产物里。

Domain services 可以承载跨实体或跨值对象的纯领域判断，但不得替代 application use case。密码 hash 属于 `application/credentials` 或 common security 原语，JWT 签发/解析和 Bearer token 处理属于 `application/tokens` 或 common security 原语，Redis session 生命周期和 token version cache/database fallback 属于 `application/sessions`、`application/validators` 或 infrastructure adapter，日志和配置读取仍属于 application、common runtime 或 infrastructure 边界。Redis key catalog 是 Redis adapter 的存储契约，放在 feature-local `infrastructure/redis` 或 owning runtime primitive 内，不是 domain service 准入样例。Domain events 只承载领域事实的数据模型；事件总线、broker、outbox、publisher、subscriber 或后台投递 worker 必须另开变更设计。

Integration adapter 可以做外部协议 DTO 转换、调用错误归一化和 client 边界处理，但不得为了 adapter 自身方便定义大接口。外部能力接口归消费侧 feature application 层所有，adapter 只负责实现。事件 producer 的业务决策归 feature application，broker envelope 和投递调用归 `integration/events`；事件 consumer 的 broker mechanics 归 `integration/events`，feature-specific 输入映射和 handler adapter 归对应 feature 的 `infrastructure/consumers`。

## 7. Common Organization

- `common/contract/errors/`：跨服务稳定应用错误码、可渲染应用错误类型和错误转换 helper。
- `common/contract/pagination/`：跨服务稳定 Cursor/Keyset 分页响应模型、分页大小边界和分页数据包装 helper。
- `common/contract/response/`：HTTP 响应信封 DTO 和默认响应消息；不承载错误码、应用错误或分页 re-export。
- `common/runtime/`：服务运行时基础能力，例如配置、日志、datastore 构造、具名 Redis/PostgreSQL Fx provider、跨服务默认 UUID 生成、进程内短 TTL 缓存、Redis key 构造规则、后台任务池、定时任务调度器、可观测性配置契约、运行时资源名和时区初始化。
- `common/http/`：HTTP/Gin 边界适配和 HTTP API 构建期辅助能力，例如 middleware、`http/binding` 请求绑定/校验失败响应适配层、无业务语义的 binder 组合/HTTP header 绑定、`http/response` 输出 helper、`http/pprof` Go runtime pprof 路由注册 helper，以及 `http/openapi` Swagger/OpenAPI 转换、规范化、序列化和 Go embed 渲染 helper；其中 Casbin 授权中间件只提供 resolver、authorizer 和 error handler 组合骨架，pprof helper 只负责挂载标准诊断端点，不决定是否开启、监听地址、鉴权、allowlist 或网关暴露策略，OpenAPI helper 不承载服务 API server、健康探针路径、认证方案描述、源码扫描范围或生成输出目录。
- `common/security/`：安全与凭证原语，例如 JWT、Bearer 传输常量、认证上下文、密码 hash helper 和无业务语义的 Casbin 三元组/enforcer 包装。
- `common/testing/`：跨模块测试基础设施和无业务语义 fixture，仅供测试代码使用；真实 PostgreSQL/Redis integration helper 放在 `testing/containers`，基础测试值生成放在 `testing/fixtures`。
- `common/validation/`：不依赖 Gin 的通用结构体校验核心、字段名解析、错误归一化和自定义 rule。

`common/runtime/workerpool` 是跨服务稳定、无业务语义的后台任务池 primitive，当前基于 ants 封装并提供并发限制、池满阻塞背压、错误日志、统计快照和 Fx 生命周期关闭。它只能承载运行时任务执行能力，不承载 feature 业务规则、业务 DTO、跨 feature 编排、事件投递语义、outbox 持久化、可靠消息语义或影响 token 有效性的 session 安全策略。Feature application 不应依赖 worker pool；需要后台清理的 infrastructure adapter 可以把它作为内部 runtime 依赖使用。高并发正式系统中后台池应按用途命名为专用 Fx 资源，例如 auth session purge 使用 `auth_session_purge_pool`，不得把多个业务场景混用到一个全局共享池。监控接入使用 worker pool 的 queued、running、waiting、failed 和 panicked 等低基数运行信号；静态 worker 数和可由 running 推导的 free workers 不导出为 Prometheus 指标。

`common/runtime/localcache` 是跨服务稳定、无业务语义的进程内短 TTL 缓存 primitive，只承载按 key 读取、写入、过期判断和主动删除能力。它不得承载 auth/user/role/permission 等 feature 缓存策略、缓存 key schema、远端缓存访问、撤销广播、容量治理或后台清理。需要强容量控制、跨实例失效或业务一致性策略时，应由消费侧 feature 或单独 runtime primitive 设计承担。

`common/runtime/rediskey` 是跨服务稳定、无业务语义的 Redis key 构造 primitive，只承载 namespace、分段拼接、prefix 和 Redis Cluster hash tag 等通用规则。它不得承载 auth/user/role/permission 等 feature key schema。Feature 私有缓存、索引、会话、投影和临时 key 由对应 `features/<feature>/infrastructure/redis` 拥有；未来真正通用的 runtime primitive（例如 rate limiter、distributed lock、idempotency）如果进入 `common/runtime/<primitive>`，由该 primitive 自己拥有自己的 key schema。

`common/runtime/id` 是跨服务稳定、无业务语义的默认 UUID 生成 primitive。当前默认实现使用 UUIDv7；调用方应通过 `NewUUID` 或 `NewUUIDString` 显式处理生成错误，测试 fixture 或静态初始化可使用 Must 版本。该包不得承载用户、角色、权限、会话等业务 ID 语义、数据库主键规则、Redis key schema、trace/span ID 生成或服务特定 ID 前缀；未来默认版本从 UUIDv7 切换到 UUIDv8 时，应集中修改该 primitive 并同步验证排序、兼容性和存储影响。

`common/runtime/scheduler` 是跨服务稳定、无业务语义的定时任务调度 primitive，当前基于 `github.com/robfig/cron/v3` v3 API 封装，并提供任务级本机防重叠、调度器全局并发限制、任务级分布式锁接口、Redis 锁实现、锁续租、优雅关闭和 `Metrics` 指标接口。Redis 锁等待由任务级 `WaitTimeout` 表达总等待上限，锁实现内部使用 `RetryPolicy` 做指数退避、最大单次间隔、可选最大尝试次数和 jitter，避免多副本锁竞争时形成固定频率 Redis 轮询。Scheduler 不得承载 feature 业务规则、业务 DTO、业务 Redis key schema、跨 feature 编排、可靠消息语义、outbox 持久化或具体 Prometheus registry wiring。需要多副本单实例执行的任务必须在任务级声明 `LockPolicy`，锁 TTL 必须解析为正值；长任务应启用续租，续租失败默认取消任务，避免锁失效后继续执行有副作用的业务逻辑。Prometheus 接入应通过服务侧实现 `Metrics` 接口完成，指标 label 中的 job key 必须是固定枚举式任务名，不得写入用户 ID、订单 ID 等高基数动态值。

`common/runtime/config` 拥有跨服务稳定的 observability 配置契约，根配置段为 `observability.metrics` 与 `observability.tracing`。该契约表达 metrics 是否开启、HTTP metrics path、是否包含 runtime 指标、tracing 是否开启、采样率、exporter 类型、OTLP endpoint 和 insecure 开关；配置加载和生产类环境安全校验由 `common/runtime/config` 统一完成。`common/runtime/observability/metrics` 已提供 Prometheus 独立 registry/provider、可选 Go runtime/process collector、统一 label key、低基数约定、Fx 友好 provider wiring、SQL DB stats collector、Redis ping collector、workerpool stats collector、scheduler Metrics adapter、runtime component status collector 和 Casbin policy reload recorder；禁用模式必须零副作用，启用模式不得使用 Prometheus 默认全局 registry，且 common package 不会自动挂载 `/metrics` 路由。用户服务通过服务级 provider 和 router 在 `observability.metrics.enabled: true` 时显式暴露 `observability.metrics.path`，默认 `/metrics`；该 endpoint 不进入 `/api/v1` feature route graph，不经过 RBAC 授权，成功 scrape 可跳过 access log。用户服务 Gin engine 通过 `common/http/middleware.HTTPServerMetrics` 记录 HTTP server RED 指标：`http_server_requests_total`、`http_server_request_duration_seconds` 和 `http_server_in_flight_requests`；业务请求使用 Gin route template 作为 `route` label，未匹配路由使用稳定 fallback，成功健康探针和成功 metrics scrape 不进入业务 RED 指标。用户服务服务级 provider 额外注册 PostgreSQL `user_db`、Redis `cache_redis`、auth session purge workerpool、RBAC policy watcher 和 Casbin policy reload 指标；这些指标只使用固定 `resource`、`pool`、`scheduler_job`、`event`、`status` 或 `reason` label，不改变 `/livez`、`/readyz`、`/startupz` 语义。用户服务 auth 和 permission feature 可以在各自 application 边界定义业务 metrics 窄接口，并由 feature Fx 通过同一个 provider 注入 Prometheus recorder；当前业务指标覆盖登录、refresh、logout、token version mismatch、session purge submit failure、RBAC policy reload/publish、watcher version mismatch 和 route diff missing/stale 数量。业务指标仍归 owning feature 拥有，不进入 `common/runtime/observability/metrics`，label 只能使用固定 `operation`、`result`、`reason`、`source`、`kind` 等枚举值，不得写入用户 ID、用户名、角色 ID、权限 ID、session ID、token version、policy version、route template、raw path、SQL、Redis key 或原始错误消息。HTTP、scheduler 和 workerpool 的 Prometheus 接入应通过 metrics adapter 或服务侧 wiring 完成，不得让 scheduler/workerpool 反向依赖具体 exporter；指标 label 不得写入用户 ID、token、trace ID、raw path、SQL、Redis key 或原始错误消息。`common/runtime/observability/tracing` 已提供 OpenTelemetry SDK tracer provider、parent-based sampler、resource attributes、W3C TraceContext/Baggage propagator、OTLP trace exporter 和 Fx 生命周期关闭；用户服务通过 OTel Gin middleware 为 HTTP 入站请求创建 server span，`exporter: none` 不创建 OTLP exporter、不连接 Collector，只生成标准 trace/span context，供日志关联和上下文传播使用，`exporter: otlp` 使用配置的 Collector endpoint 导出 span。Observability 包不得承载用户服务业务指标、dashboard、告警规则、部署清单、feature span 名称或业务 attribute。

`common/security/casbin` 是跨服务稳定、无业务语义的 Casbin 授权 primitive，只承载 `subject/object/action` 请求三元组、通用 enforcer 调用包装、拒绝和未配置错误。`common/http/middleware` 中的 Casbin 授权中间件只负责 Gin 调用骨架，必须通过服务侧 resolver、authorizer 和 error handler 注入业务语义。它不得承载 user-service 的 `user:<user_uuid>`、`role:<role_uuid>` subject schema、权限目录、policy loader、super admin baseline、route diff 或服务特定错误码。

`common/http/openapi` 是跨服务稳定、无服务业务语义的 OpenAPI 构建期辅助包，只负责 Swagger/OpenAPI 文档转换、调用方显式传入的规范化、JSON/YAML 序列化和 Go embed 源码渲染。服务自己的 OpenAPI 生成脚本或薄 wrapper 负责 `swag init` 源码扫描范围、API server URL、根路径健康检查、认证方案名称和描述、输出目录与生成文件包名。用户服务生成产物继续位于 `user-service/docs/`，运行时 OpenAPI UI/JSON 路由继续由 `user-service/internal/router/openapi.go` 拥有。

新增共享代码进入 `common` 前必须满足跨服务稳定、无业务语义、边界清晰。服务独有规则、DTO 映射、infrastructure adapter 行为或只为未来可能复用的 helper 应保留在对应服务模块内。未来 `common/runtime/eventbus` 只有在至少两个服务需要同一稳定 runtime primitive，且 API 不含用户服务业务语义时才可新增；它不得承载 user-service event name、feature port、业务 DTO 或 speculative broker abstraction。未来 outbox 若要进入 `common/runtime/outbox`，必须先通过单独变更设计 transaction boundary、存储模型、投递 worker、重试、幂等和失败策略；在该设计前不得新增 outbox table、Ent hook、transaction wrapper 或 dispatcher。

## 8. Data Model

`user-service/ent/schema/user.go` 定义当前用户模型：

| 字段 | 约束 |
|---|---|
| `id` | `int64`、唯一、不可变 |
| `user_id` | UUID、唯一、不可变、对外用户标识 |
| `name` | 非空、最大 128 |
| `username` | 非空、唯一、最大 255 |
| `password` | 非空、密码哈希 |
| `token_version` | 默认 `1` |
| `active` | 默认 `true` |
| `created_at` | 默认当前时间、不可变 |
| `updated_at` | 默认当前时间、更新时自动刷新 |

## 9. Infrastructure

- 配置加载由 `common/runtime/config/loader.go` 负责，支持 YAML 文件和 `AEGISCORE_` 环境变量覆盖。
- 可观测性配置由 `common/runtime/config` 的 `observability.metrics` 和 `observability.tracing` 承载；`common/runtime/observability/metrics` 的 provider 可按配置创建 Prometheus 独立 registry 并可选注册 Go runtime/process collector，用户服务在 `observability.metrics.enabled: true` 时通过服务级 router 显式挂载 `observability.metrics.path`，默认 `/metrics`。该 endpoint 不经过 RBAC 授权；是否需要网络侧保护由部署层决定。用户服务 HTTP 入站 RED 指标由 `common/http/middleware.HTTPServerMetrics` 采集，route label 使用 Gin route template，未匹配路由使用稳定 fallback，成功健康探针和成功 metrics scrape 默认过滤。用户服务服务级 provider 注册 `user_db` PostgreSQL pool stats、`cache_redis` Redis ping 状态、auth session purge workerpool stats、RBAC policy watcher 状态和 Casbin policy reload 状态；auth 和 permission feature 额外注册 feature-owned business metrics，覆盖认证会话、token version mismatch、RBAC policy 同步和 route diff 诊断数量。这些指标不改变 health probe 语义，不暴露 DSN、SQL、Redis key、错误消息全文或业务实体 ID。`common/runtime/observability/tracing` 的 `exporter: none` provider 可在无 Collector 环境创建和关闭；`exporter: otlp` 会连接配置的 OTLP Collector 并导出 span；OTel Gin middleware 会为 HTTP 入站请求生成标准 OTel trace ID 与 span ID。
- PostgreSQL 使用 `postgres.<name>` 命名实例配置；用户服务当前只声明并连接 `postgres.user_db`。配置中出现其他 PostgreSQL 命名实例不代表用户服务会自动连接或迁移它们。
- Redis 使用 `redis.<name>` 命名实例配置；用户服务当前声明并连接 `redis.cache_redis`。
- 默认 UUID 生成使用 `common/runtime/id`；业务 ID 创建优先使用 `NewUUID` 或 `NewUUIDString` 并显式处理错误。
- 进程内短 TTL 缓存使用 `common/runtime/localcache`；消费侧负责选择 TTL、key 类型和值类型，并负责业务失效策略。
- Redis key 使用 `common/runtime/rediskey` 统一 namespace、分段拼接、prefix 和 Redis Cluster hash tag 构造规则；具体 key schema 由 owning feature 的 `infrastructure/redis` 或 owning runtime primitive 管理。
- 定时任务调度能力使用 `common/runtime/scheduler`；服务侧负责按用途创建调度器、注入既有 Redis client 到 `RedisLocker`、实现监控指标适配并在 Fx 生命周期中启动和关闭调度器。任务很多或资源敏感时可配置全局并发上限；是否需要分布式锁由每个任务的 `LockPolicy` 声明，不使用全局单开关替代任务级语义。
- 用户服务的 Redis/PostgreSQL named resource、JWT service、Gin engine 和 Ent clients 由 `user-service/internal/providers/` 提供，其中 Ent clients 由 `providers/ent.go` 基于具名 `*sql.DB` 构建。
- 用户服务认证 Redis adapter 对登录和 refresh rotation 的 refresh session 写入执行同步 Redis Lua 原子操作，并按 application 传入的每用户活跃 session 上限裁剪最旧 session；该裁剪影响 refresh token 可续期能力，不通过 worker pool 异步执行。
- 用户服务认证 Redis adapter 使用 `common/runtime/workerpool` 管理退出全部设备后的 detached session 后台物理清理；该 worker pool 只负责受控后台执行，不是 MQ、eventbus、outbox、通用 job system、可靠投递框架或 session 上限策略执行器。
- 用户服务的外部系统防腐层边界位于 `user-service/internal/integration/`；其中 `integration/grpc` 只表示出站外部 gRPC client adapter，不表示本服务入站 gRPC transport；`integration/events` 只表示外部事件系统协议 adapter，不表示 feature consumer handler 或业务事件编排；当前没有 order、payment 等真实外部 client，也没有 Kafka、RabbitMQ、NATS、Redis Stream 等 broker dependency；当前也没有事件总线、outbox、publisher、subscriber、consumer handler 或异步投递 worker。
- 部署资产位于 `deployments/`：用户服务 Dockerfile 位于 `deployments/docker/user-service.Dockerfile`，并要求从仓库根目录执行 build；`deployments/compose/` 承载本地依赖或本地服务启动配置，`deployments/k8s/` 承载 Kubernetes YAML，`deployments/helm/` 承载 Helm chart，`deployments/observability/` 承载 Prometheus/Grafana dashboard 和 alert 示例。部署清单、配置模板、Secret/ConfigMap 示例、探针、资源配额、服务暴露方式、dashboard 和告警规则都留在 `deployments/` 或对应部署模板中，不迁入 `user-service/internal/shared`、`common` 或 feature 代码目录。
- 日志基于 Zap，由 `common/runtime/logger` 提供底层构造和 Fx provider；HTTP trace 传播使用 W3C `traceparent` / `tracestate`，服务端不承诺返回 trace header；`common/runtime/logger` context helper 从当前 OTel span context 自动追加 `trace_id` 与 `span_id`，无有效 span context 时不伪造 trace/span 字段。
- HTTP access log 标准字段为 `trace_id`、`user_id`、`client_ip`、`method`、`path`、`status`、`latency_ms`；认证失败安全事件日志应额外记录 `user_agent`，但不得记录 password、token、Authorization header、Cookie 或原始请求体。
- 用户服务 HTTP 探针由 `internal/router/health.go` 拥有：`/livez` 只表示 Gin 进程可响应请求；`/readyz` 和 `/startupz` 由 `internal/providers/health.go` 注入 PostgreSQL `user_db`、Redis `cache_redis`、Casbin `LastError` 和 RBAC policy watcher 状态检查，失败时返回 HTTP 503 且不暴露 DSN、token、Cookie、SQL 或 stacktrace。
- 代码注释统一使用中文，函数和方法注释必须使用中文；必要的协议名、库名、HTTP/JWT/Redis/PostgreSQL/Ent/Fx/Gin/trace-id 等技术术语可保留英文。人工维护源码不得新增英文注释；生成代码和第三方代码不为翻译注释而手写修改。
- Log 日志消息内容必须全部使用英文，日志字段名使用稳定英文 snake_case。日志级别必须匹配场景严重性：`Debug` 用于生命周期细节和调试信息，`Info` 用于服务启动停止、资源连接关闭和重要成功业务动作，`Warn` 用于预期业务拒绝、客户端输入问题、认证拒绝、缓存降级和非致命冲突，`Error` 用于系统异常、外部依赖失败、数据访问失败、后台任务失败和 panic recover。业务日志优先使用 `common/runtime/logger` context helper，避免丢失 trace/span context。

## 10. Database Migrations

- 用户服务使用服务内迁移目录 `user-service/migrations/`，Atlas 配置位于 `user-service/migrations/atlas.hcl`。
- Ent schema 是期望数据库结构来源；开发期通过 `go generate ./ent` 生成 Ent 代码，通过 `./scripts/migrate-diff.sh <name>` 生成 SQL migration，并通过 `./scripts/migrate-validate.sh` 校验 `atlas.sum`。
- 运行时不得通过 `client.Schema.Create(ctx)` 自动创建或修改 schema；生产迁移应由 CI/CD release job 或独立 migration Job 在 HTTP runtime rollout 前执行。容器 `entrypoint.sh` 仅在显式设置 `RUN_MIGRATIONS=true` 时作为简单部署或兼容场景执行迁移，不作为多副本生产发布的默认编排手段。
- 迁移执行应面向用户服务拥有的 `user_db`，不得因为配置中存在其他数据库配置而迁移非目标数据库。

## 11. Generated Code

`user-service/ent/` 大多是 Ent 生成代码。业务变更应优先修改 `user-service/ent/schema/`，然后在 `user-service/` 运行 `go generate ./ent` 重新生成。不要直接编辑生成代码来表达领域变更。

## 12. Current Constraints

- 当前 HTTP API 暴露 `/livez`、`/readyz`、`/startupz` 健康探针、OpenAPI 文档、用户资料、认证会话、角色管理、权限目录、用户有效权限查询、只读 route diff 和 RBAC 授权保护的业务接口。
- 当前没有真实 gRPC API、`.proto` schema、protobuf generated code 或 gRPC server runtime；如未来暴露入站 gRPC API，应先在对应 feature 的 `transport/grpc` 建立真实 API 设计，并单独设计服务级 runtime wiring。
- 当前没有真实外部系统 client；`internal/integration` 只声明 HTTP、gRPC 和 events 防腐层边界。
- 当前没有真实 MQ/broker、eventbus、outbox、producer、subscriber、consumer handler、后台投递 worker 或事件驱动事务语义；事件发布、事件消费和可靠投递能力都需要单独变更设计。
- 配置样例可能包含未来资源配置，但用户服务只初始化自己显式声明的 Redis/PostgreSQL named resources。
- 启动服务需要 PostgreSQL 和 Redis 可连接；纯单元测试应避免依赖真实外部服务，集成测试需要显式说明依赖。
