# Architecture

## 1. Overview

AegisCore 是 Go 1.26 workspace，当前包含共享基础模块 `common` 和用户服务模块 `user-service`。用户服务通过 Cobra 提供 CLI，通过 Uber Fx 组装依赖，通过 Gin 暴露 HTTP API，通过 Ent 访问 PostgreSQL，并通过 Atlas 维护服务内 SQL migration。

本文件和根目录 `AGENTS.md` 是仓库结构与分层规则的唯一长期规则源。仓库不再维护 OpenSpec/OPSX 工件。

## 2. Module Boundaries

| 模块 | 责任 | 关键位置 |
|---|---|---|
| `common` | 跨服务稳定契约与基础能力；不得承载服务特定 helper 或业务语义 | `common/contract/`, `common/runtime/`, `common/http/`, `common/security/`, `common/validation/` |
| `user-service` | 用户服务运行时、用户资料与认证会话 feature、外部系统防腐层边界、Ent schema、Atlas migration、Swagger 文档 | `user-service/cmd/`, `user-service/internal/`, `user-service/internal/integration/`, `user-service/ent/`, `user-service/migrations/`, `user-service/docs/` |
| `deployments` | 本地和生产部署资产 | `deployments/compose/`, `deployments/docker/`, `deployments/k8s/`, `deployments/helm/` |

仓库根目录是 workspace，不是业务 Go module。运行 Go 命令时通常进入 `common/` 或 `user-service/`。

## 3. Runtime Flow

1. `user-service/cmd/main.go` 创建 `aegiscore-user-services` CLI，并注册 `serve` 子命令。
2. `serve` 调用 `bootstrap.NewApp(configPath)` 创建 Fx 应用。
3. `user-service/internal/bootstrap.AppModule` 导入共享 runtime module、feature modules、`providers.Module`，并提供 HTTP server 生命周期。
4. `user-service/internal/providers.Module` 显式提供 Redis/PostgreSQL named providers、Ent clients、JWT service、Gin engine 和 HTTP route registration。
5. User/Auth feature modules 自己组装 feature-local infrastructure adapter、application service 或 command/query use case 和 HTTP controller。
6. `user-service/internal/providers/routes.go` 适配依赖并调用 `router.RegisterUserServiceHTTPRoutes`；`router.go` 负责 route graph 总装和 `/api/v1` 分组，`health.go` 注册 `/healthz`，`swagger.go` 注册 Swagger UI 和文档重定向。
7. Fx lifecycle 启动 HTTP server，并在进程收到中断或 SIGTERM 时优雅关闭。

`aegiscore-user-services` 是当前运行时 CLI/service name，不是仓库目录名或 Go module path；代码位置和 module path 统一使用 `user-service`。

## 4. HTTP Request Flow

| 步骤 | 代码位置 | 行为 |
|---|---|---|
| 中间件链 | `user-service/internal/providers/gin.go` | 创建 Gin engine，注册 trace-id、panic recovery、request logging、CORS |
| 路由 provider | `user-service/internal/providers/routes.go` | 将 Fx 依赖适配为 router route params |
| 路由总装 | `user-service/internal/router/router.go`、`health.go`、`swagger.go` | `router.go` 创建 public/protected route groups 并总装 route graph，`health.go` 注册 `/healthz`，`swagger.go` 注册 Swagger UI 和文档重定向 |
| 参数解析 | `features/*/transport/http/controller.go` | 绑定 HTTP DTO，执行边界校验，并映射为 command/query |
| 业务调用 | `features/*/application/` | 编排用户资料或认证会话用例；用户资料 feature 的读写用例分别位于 `application/query` 与 `application/command`，认证会话 feature 的登录、刷新、强制改密和登出用例位于 `application/command` |
| 数据访问 | `features/*/infrastructure/postgres/`, `features/*/infrastructure/redis/` | 使用 Ent 或 Redis 访问持久化细节，转换存储层错误 |
| 响应输出 | `common/http/response/` + `common/contract/response/` | 通过 Gin writer 输出统一 `success/code/message/data` 信封，并复用稳定错误码与分页契约 |

## 5. Feature-First Organization

服务内业务代码按 feature 组织在 `user-service/internal/features/<feature>/`。当前稳定 feature 包括：

- `user`：用户资料创建、查询和分页列表。
- `auth`：登录、刷新、强制改密、退出当前设备、退出全部设备。

每个 feature 使用以下分层：

| 目录 | 责任 |
|---|---|
| `application/` | service、commands、queries、ports、use case mapper 和业务编排；可按 feature 需要细分为 `command/`、`query/`、`validators/` 和稳定组件包。Auth 当前使用 `command/` 承载登录、刷新、强制改密和登出 use case，使用 `validators/` 承载 transport-neutral 输入辅助，使用 `tokenversion/` 承载 token version 撤销校验和 cache/database fallback 策略 |
| `domain/` | 领域实体、值对象、枚举、领域错误和纯业务规则 |
| `transport/http/` | Gin controller、route registration、HTTP request/response DTO、Swagger 文档模型、HTTP DTO validation 和边界映射 |
| `infrastructure/postgres/` | Ent/PostgreSQL adapter 和 predicate 构造 |
| `infrastructure/redis/` | Redis adapter；仅在 feature 需要 Redis 时存在 |
| `fx.go` | Feature-local Fx module，组装 application、transport 和 infrastructure provider |

不要新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包。跨 feature 的共享业务代码也不要默认放到 `internal/shared`；只有当能力已被至少两个 feature 真实消费、边界稳定、且不能归入 `common` 时，才可以新增，并需在本文件补充 owner、准入理由和禁止事项。

服务级 provider 统一放在 `user-service/internal/providers`。该包只负责把共享 runtime、common security、Gin、router 和 Ent 适配为用户服务 Fx 依赖；不得承载 feature 业务逻辑、HTTP route 定义或跨服务共享基础能力。`internal/bootstrap` 只负责 `fx.New`、顶层 `AppModule` 总装和 HTTP server 生命周期。

外部系统防腐层统一放在 `user-service/internal/integration`，并按 `http/`、`grpc/`、`events/` 分类。该边界只在有真实外部系统调用时承载协议 client adapter、外部 DTO 映射、外部错误语义归一化和传输细节；当前没有真实外部调用时只保留 README 或 package doc。`integration` 不属于 feature 内部业务编排，不拥有用例流程、登录状态机、跨 store 事务、HTTP controller 或本服务持久化访问。Feature application service 或 command/query 用例仍通过消费侧 ports 表达外部能力需求，integration adapter 只实现这些最小接口。

## 6. Dependency Rules

| 层 | 可以依赖 | 禁止依赖 |
|---|---|---|
| `domain` | 标准库、稳定值对象 | Gin、Ent、Redis、config、response envelope |
| `application` | `domain`、消费侧端口接口、common 安全原语 | Gin、Ent、Redis、HTTP binder |
| `transport/http` | `application`、Gin、response envelope、feature-local HTTP DTO 和 validation | Ent、Redis、SQL |
| `infrastructure/postgres` | Ent、SQL、application ports、domain | Gin、HTTP response |
| `infrastructure/redis` | Redis client、application ports、domain | Gin、HTTP response |
| `integration/*` | 外部 SDK/client、feature application ports、domain、common runtime/security 原语 | Gin response、Ent、feature service 业务编排、service-owned persistence adapter |
| `fx.go` | Fx、feature 内部包 | 业务逻辑 |

Ports 由消费侧 feature 拥有。Infrastructure adapter 只实现 application 层定义的最小接口，不为了自身方便定义大接口。

Controller 必须把 HTTP DTO 映射为 application command/query 后再调用 service 或用例。Service 和 command/query 用例不接收 HTTP request/response DTO，也不导入 Gin、Ent predicate、Redis client 或 HTTP binder。

Ent predicate 构造封装在 `infrastructure/postgres` 内。Adapter 可以做字段裁剪、模型转换和存储错误转换，但不得承载复杂业务编排、登录状态机、密码校验、token 签发、跨 store 事务编排或 HTTP 错误映射。

Integration adapter 可以做外部协议 DTO 转换、调用错误归一化和 client 边界处理，但不得为了 adapter 自身方便定义大接口。外部能力接口归消费侧 feature application 层所有，adapter 只负责实现。

## 7. Common Organization

- `common/contract/errors/`：跨服务稳定应用错误码、可渲染应用错误类型和错误转换 helper。
- `common/contract/pagination/`：跨服务稳定 Cursor/Keyset 分页响应模型、分页大小边界和分页数据包装 helper。
- `common/contract/response/`：HTTP 响应信封 DTO 和默认响应消息；不承载错误码、应用错误或分页 re-export。
- `common/runtime/`：服务运行时基础能力，例如配置、日志、datastore 构造、具名 Redis/PostgreSQL Fx provider、运行时资源名和时区初始化。
- `common/http/`：HTTP/Gin 边界适配，例如 middleware、`http/binding` 请求绑定/校验失败响应适配层和 `http/response` 输出 helper。
- `common/security/`：安全与凭证原语，例如 JWT、Bearer 传输常量、认证上下文和密码 hash helper。
- `common/testing/`：跨模块测试基础设施和无业务语义 fixture，仅供测试代码使用；真实 PostgreSQL/Redis integration helper 放在 `testing/containers`，基础测试值生成放在 `testing/fixtures`。
- `common/validation/`：不依赖 Gin 的通用结构体校验核心、字段名解析、错误归一化和自定义 rule。

新增共享代码进入 `common` 前必须满足跨服务稳定、无业务语义、边界清晰。服务独有规则、DTO 映射、infrastructure adapter 行为或只为未来可能复用的 helper 应保留在对应服务模块内。

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
- PostgreSQL 使用 `postgres.<name>` 命名实例配置；用户服务当前声明并连接 `postgres.user_db` 与 `postgres.common_db`。
- Redis 使用 `redis.<name>` 命名实例配置；用户服务当前声明并连接 `redis.cache_redis`。
- 用户服务的 Redis/PostgreSQL named resource、JWT service、Gin engine 和 Ent clients 由 `user-service/internal/providers/` 提供，其中 Ent clients 由 `providers/ent.go` 基于具名 `*sql.DB` 构建。
- 用户服务的外部系统防腐层边界位于 `user-service/internal/integration/`；当前没有 order、payment 等真实外部 client，也没有 Kafka、RabbitMQ、NATS 等 broker dependency。
- 日志基于 Zap，由 `common/runtime/logger` 提供底层构造和 Fx provider；HTTP trace header 为 `X-Trace-ID`，Gin context key 为 `trace_id`，日志字段统一为 `trace-id`。

## 10. Database Migrations

- 用户服务使用服务内迁移目录 `user-service/migrations/`，Atlas 配置位于 `user-service/atlas.hcl`。
- Ent schema 是期望数据库结构来源；开发期通过 `go generate ./ent` 生成 Ent 代码，通过 `./scripts/migrate-diff.sh <name>` 生成 SQL migration，并通过 `./scripts/migrate-validate.sh` 校验 `atlas.sum`。
- 运行时不得通过 `client.Schema.Create(ctx)` 自动创建或修改 schema；迁移应由 CI/CD release job 或容器 entrypoint 在 HTTP runtime 启动前执行。
- 迁移执行应面向用户服务拥有的 `user_db`，不得因为配置中存在其他数据库配置而迁移非目标数据库。

## 11. Generated Code

`user-service/ent/` 大多是 Ent 生成代码。业务变更应优先修改 `user-service/ent/schema/`，然后在 `user-service/` 运行 `go generate ./ent` 重新生成。不要直接编辑生成代码来表达领域变更。

## 12. Current Constraints

- 当前 HTTP API 暴露健康检查、创建用户、用户列表、按 `user_id` 查询用户和认证会话接口。
- 当前没有真实外部系统 client；`internal/integration` 只声明 HTTP、gRPC 和 events 防腐层边界。
- 配置样例可能包含未来资源配置，但用户服务只初始化自己显式声明的 Redis/PostgreSQL named resources。
- 启动服务需要 PostgreSQL 和 Redis 可连接；纯单元测试应避免依赖真实外部服务，集成测试需要显式说明依赖。
