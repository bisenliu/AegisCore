# Architecture

## 1. Overview

AegisCore 当前是 Go 1.26 workspace，包含共享基础设施模块 `common` 和用户服务模块 `user-services`。用户服务通过 Cobra 提供 CLI 命令，通过 Uber Fx 组装依赖，通过 Gin 提供 HTTP API，通过 Ent 访问 PostgreSQL，并通过 Atlas 维护服务内 SQL migration。

## 2. Module Boundaries

| 模块 | 责任 | 关键位置 |
|---|---|---|
| `common` | 跨服务共享配置、Zap 日志、Redis/PostgreSQL 单实例创建能力、HTTP 中间件、响应信封、错误模型 | `common/config/`, `common/infrastructure/`, `common/middleware/`, `common/response/` |
| `user-services` | 用户服务运行时、用户 HTTP API、用户领域访问、Ent schema、Atlas migration | `user-services/cmd/`, `user-services/internal/`, `user-services/ent/`, `user-services/migrations/` |
| `openspec` | OPSX/OpenSpec 规则、主规格和后续变更 artifacts | `openspec/config.yaml`, `openspec/specs/` |

## 3. Runtime Flow

1. `user-services/cmd/main.go` 创建 `aegiscore-user-services` CLI，并注册 `serve` 子命令。
2. `serve` 调用 `bootstrap.NewApp(configPath)` 创建 Fx 应用。
3. 用户服务启动装配显式提供 `commoninfra.NewConfig` 和 `commoninfra.NewLogger`；Redis/PostgreSQL 由 common 提供单实例创建与 lifecycle helper。
4. `user-services/internal/bootstrap.UserServiceModule` 显式声明 `cache_redis`、`user_db`、`common_db`，并提供 Ent clients、repository、service、controller、Gin engine、HTTP server。
5. `RegisterRoutes` 将 `/healthz`、`/api/v1/users` 和 `/api/v1/users/:user_id` 注册到 Gin engine。
6. Fx 生命周期启动 HTTP server，并在进程收到中断或 SIGTERM 时优雅关闭。

## 4. HTTP Request Flow

| 步骤 | 代码位置 | 行为 |
|---|---|---|
| 中间件链 | `user-services/internal/bootstrap/bootstrap.go` | 注册 trace-id、panic recovery、request logging、CORS；trace-id 先于日志和 recovery 执行 |
| 路由匹配 | `user-services/internal/router/router.go` | 匹配 `/healthz` 或 `/api/v1/users/:user_id` |
| 参数解析 | `user-services/internal/controller/user_controller.go` | 将 path `user_id` 校验为 UUID 字符串 |
| 业务调用 | `user-services/internal/service/user_service.go` | 调用 repository 并映射为 `dto.UserResponse` |
| 数据访问 | `user-services/internal/repository/user_repository.go` | 使用 Ent client 查询用户，not found 转为应用错误 |
| 响应输出 | `common/response/response.go` | 统一输出 `success/code/message/data` 信封 |

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

- 配置加载由 `common/config/loader.go` 负责，支持 YAML 文件和 `AEGISCORE_` 环境变量覆盖；加载阶段只做读取、覆盖和反序列化，不做 required/range 字段校验。
- PostgreSQL 使用 `postgres.<name>` 命名实例配置；用户服务当前声明并连接 `postgres.user_db` 与 `postgres.common_db`，不因存在 `postgres.pay_db` 而初始化支付连接池。
- Redis 使用 `redis.<name>` 命名实例配置；用户服务当前声明并连接 `redis.cache_redis`，不因存在 `redis.queue_redis` 而初始化队列 Redis。
- Ent clients 由 `user-services/internal/entclient/provider.go` 基于具名 `*sql.DB` 构建。
- 日志基于 Zap，由 `common/logger` 与 `common/infrastructure/logger.go` 提供；HTTP trace header 为 `X-Trace-ID`，Gin context key 为 `trace_id`，日志字段统一为 `trace-id`。

## 7. Database Migrations

- 用户服务使用服务内迁移目录 `user-services/migrations/`，Atlas 配置位于 `user-services/atlas.hcl`。
- Ent schema 是期望数据库结构来源；开发期通过 `./scripts/migrate-diff.sh <name>` 生成 SQL migration，并通过 `./scripts/migrate-validate.sh` 校验 `atlas.sum`。
- 运行时不得通过 `client.Schema.Create(ctx)` 自动创建或修改 schema；迁移应由 CI/CD release job 或容器 entrypoint 在 HTTP runtime 启动前执行。
- 迁移执行应面向用户服务拥有的 `user_db`，不得因为配置中存在 `pay_db` 或 `common_db` 而迁移非目标数据库。

## 8. Generated Code

`user-services/ent/` 大多是 Ent 生成代码。业务变更应优先修改 `user-services/ent/schema/`，然后运行 `go generate ./ent` 重新生成。不要直接编辑生成代码来表达领域变更。

## 9. Current Constraints

- 当前 HTTP API 暴露健康检查、创建用户、用户列表、按 `user_id` 查询用户和认证会话接口。
- 配置样例包含 `postgres.pay_db`，但当前用户服务只声明 `postgres.user_db`、`postgres.common_db` 和 `redis.cache_redis`。
- 启动服务需要 PostgreSQL 和 Redis 可连接；本地纯单元测试应避免依赖真实外部服务，或显式说明集成测试要求。
