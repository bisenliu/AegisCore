# Architecture

## 1. Overview

AegisCore 当前是 Go 1.26 workspace，包含共享基础设施模块 `common` 和用户服务模块 `user-services`。用户服务通过 Cobra 提供 CLI 命令，通过 Uber Fx 组装依赖，通过 Gin 提供 HTTP API，通过 Ent 访问 PostgreSQL，并通过 Atlas 维护服务内 SQL migration。

## 2. Module Boundaries

| 模块 | 责任 | 关键位置 |
|---|---|---|
| `common` | 跨服务稳定契约与基础能力模块，按 contract、runtime、http、security、validation 分类承载共享能力；不得作为服务特定 helper 的兜底目录 | `common/contract/response/`, `common/runtime/`, `common/http/`, `common/security/`, `common/validation/` |
| `user-services` | 用户服务运行时、feature-local 用户 HTTP API 和认证会话能力、Ent schema、Atlas migration | `user-services/cmd/`, `user-services/internal/features/`, `user-services/internal/bootstrap/`, `user-services/internal/router/`, `user-services/ent/`, `user-services/migrations/` |
| `openspec` | OPSX/OpenSpec 规则、主规格和后续变更 artifacts | `openspec/config.yaml`, `openspec/specs/` |

## 3. Runtime Flow

1. `user-services/cmd/main.go` 创建 `aegiscore-user-services` CLI，并注册 `serve` 子命令。
2. `serve` 调用 `bootstrap.NewApp(configPath)` 创建 Fx 应用。
3. 用户服务启动装配显式提供共享配置和日志 provider；Redis/PostgreSQL 由 common runtime datastore helper 创建。
4. `user-services/internal/bootstrap.AppModule` 显式声明 `cache_redis`、`user_db`、`common_db`，并提供 Ent clients、feature-local user/auth store、app service、controller、Gin engine、HTTP server。
5. `RegisterRoutes` 将 `/healthz`、Swagger、`/api/v1/auth/*` 和 `/api/v1/users*` 注册到 Gin engine。
6. Fx 生命周期启动 HTTP server，并在进程收到中断或 SIGTERM 时优雅关闭。

## 4. HTTP Request Flow

| 步骤 | 代码位置 | 行为 |
|---|---|---|
| 中间件链 | `user-services/internal/bootstrap/gin.go` | 注册 trace-id、panic recovery、request logging、CORS；trace-id 先于日志和 recovery 执行 |
| 路由匹配 | `user-services/internal/router/router.go` | 匹配 `/healthz`、`/api/v1/auth/*` 或 `/api/v1/users*` |
| 参数解析 | `user-services/internal/features/user/app/controller.go`, `user-services/internal/features/auth/app/controller.go` | 绑定 feature API DTO，执行边界校验，并映射为 command/query |
| 业务调用 | `user-services/internal/features/user/app/service.go`, `user-services/internal/features/auth/app/service.go` | 编排用户资料或认证会话用例，并映射应用错误 |
| 数据访问 | `user-services/internal/features/user/store/postgres/user_store.go`, `user-services/internal/features/auth/store/postgres/credential_store.go`, `user-services/internal/features/auth/store/redis/session_store.go` | 使用 Ent 或 Redis 访问持久化细节，not found 转为 capability 领域错误 |
| 响应输出 | `common/contract/response/response.go` | 统一输出 `success/code/message/data` 信封 |

## 5. Data Model

`user-services/ent/schema/user.go` 定义当前用户模型：

| 字段 | 约束 |
|---|---|
| `id` | `int64`、唯一、不可变 |
| `user_id` | UUID、唯一、不可变、对外用户标识 |
| `name` | 非空、最大 128 |
| `username` | 非空、唯一、最大 255 |
| `password` | 非空、Argon2id 密码哈希 |
| `token_version` | 默认 `1` |
| `active` | 默认 `true` |
| `created_at` | 默认当前时间、不可变 |
| `updated_at` | 默认当前时间、更新时自动刷新 |

## 6. Infrastructure

- 配置加载由 `common/runtime/config/loader.go` 负责，支持 YAML 文件和 `AEGISCORE_` 环境变量覆盖；加载阶段只做读取、覆盖和反序列化，不做 required/range 字段校验。
- PostgreSQL 使用 `postgres.<name>` 命名实例配置；用户服务当前声明并连接 `postgres.user_db` 与 `postgres.common_db`，不因存在 `postgres.pay_db` 而初始化支付连接池。
- Redis 使用 `redis.<name>` 命名实例配置；用户服务当前声明并连接 `redis.cache_redis`，不因存在 `redis.queue_redis` 而初始化队列 Redis。
- Ent clients 由 `user-services/internal/bootstrap/ent.go` 基于具名 `*sql.DB` 构建。
- 日志基于 Zap，由 `common/runtime/logger` 与 `common/runtime/loggerfx` 提供；HTTP trace header 为 `X-Trace-ID`，Gin context key 为 `trace_id`，日志字段统一为 `trace-id`。

## 7. Feature Organization

- `user-services/internal/features/user/api/`：用户资料 HTTP request/response DTO 和 Swagger 文档模型。
- `user-services/internal/features/user/app/`：用户 controller、service、commands、ports 和 use case mapper。
- `user-services/internal/features/user/domain/`：用户实体、状态枚举和领域错误。
- `user-services/internal/features/user/store/postgres/`：用户资料 Ent/PostgreSQL adapter 和 Ent predicate 构造。
- `user-services/internal/features/auth/api/`：认证登录、刷新、改密、登出相关 HTTP DTO。
- `user-services/internal/features/auth/app/`：认证 controller、service、credential/token/session 组件、commands 和 ports。
- `user-services/internal/features/auth/domain/`：认证凭据、认证会话、会话吊销结果、认证领域错误和 Redis key 业务语义。
- `user-services/internal/features/auth/store/postgres/`：认证凭据和 token version 的 Ent/PostgreSQL adapter。
- `user-services/internal/features/auth/store/redis/`：Refresh Token 会话和 token version cache 的 Redis adapter。

## 8. Common Organization

- `common/contract/`：跨服务外部契约，例如 `response` 响应信封、错误码和分页响应模型。
- `common/runtime/`：服务运行时基础能力，例如配置、日志、datastore 构造、具名 Redis/PostgreSQL Fx provider、运行时资源名、时区初始化和 Fx lifecycle helper。
- `common/http/`：HTTP/Gin 边界适配，例如 middleware 和 Gin 请求绑定/校验失败响应适配层。
- `common/security/`：安全与凭证原语，例如 JWT、Bearer 传输常量、认证上下文和 Argon2id 密码 helper。
- `common/validation/`：不依赖 Gin 的通用结构体校验核心、字段名解析、错误归一化和自定义 rule。
- 新增共享代码进入 `common` 前必须先定位 capability；服务独有规则、DTO 映射、repository 行为或只为未来可能复用的 helper 应保留在对应服务模块内。

## 9. Database Migrations

- 用户服务使用服务内迁移目录 `user-services/migrations/`，Atlas 配置位于 `user-services/atlas.hcl`。
- Ent schema 是期望数据库结构来源；开发期通过 `./scripts/migrate-diff.sh <name>` 生成 SQL migration，并通过 `./scripts/migrate-validate.sh` 校验 `atlas.sum`。
- 运行时不得通过 `client.Schema.Create(ctx)` 自动创建或修改 schema；迁移应由 CI/CD release job 或容器 entrypoint 在 HTTP runtime 启动前执行。
- 迁移执行应面向用户服务拥有的 `user_db`，不得因为配置中存在 `pay_db` 或 `common_db` 而迁移非目标数据库。

## 10. Generated Code

`user-services/ent/` 大多是 Ent 生成代码。业务变更应优先修改 `user-services/ent/schema/`，然后运行 `go generate ./ent` 重新生成。不要直接编辑生成代码来表达领域变更。

## 11. Current Constraints

- 当前 HTTP API 暴露健康检查、创建用户、用户列表、按 `user_id` 查询用户和认证会话接口。
- 配置样例包含 `postgres.pay_db`，但当前用户服务只声明 `postgres.user_db`、`postgres.common_db` 和 `redis.cache_redis`。
- 启动服务需要 PostgreSQL 和 Redis 可连接；本地纯单元测试应避免依赖真实外部服务，或显式说明集成测试要求。
