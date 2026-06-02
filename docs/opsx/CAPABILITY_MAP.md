# Capability Map

本地图将长期稳定能力映射到代码模块和 OpenSpec 主规格。提出或实现变更前，先定位相关 capability。

## 1. Capabilities

| Capability | 业务说明 | 主要代码位置 | 主规格 | 状态 |
|---|---|---|---|---|
| `user-profile-query` | 通过用户 ID 查询用户资料并返回统一 API 响应 | `user-services/internal/router/router.go`, `user-services/internal/controller/user_controller.go`, `user-services/internal/service/user_service.go`, `user-services/internal/repository/user_repository.go`, `user-services/ent/schema/user.go` | `openspec/specs/user-profile-query/spec.md` | ready |
| `http-service-runtime` | 通过 CLI 启动 HTTP 服务，注册中间件和路由，并支持优雅关闭 | `user-services/cmd/main.go`, `user-services/internal/bootstrap/bootstrap.go`, `user-services/internal/router/router.go` | `openspec/specs/http-service-runtime/spec.md` | ready |
| `shared-infrastructure` | 加载配置，提供 Zap 日志，并支持服务侧声明具名 Redis/PostgreSQL/Ent 运行时依赖 | `common/config/`, `common/infrastructure/`, `common/logger/`, `user-services/internal/bootstrap/`, `user-services/internal/entclient/provider.go` | `openspec/specs/shared-infrastructure/spec.md` | ready |
| `api-response-contract` | 统一 HTTP 成功/失败信封、错误码和应用错误映射 | `common/response/`, `common/middleware/recovery.go`, `user-services/internal/controller/user_controller.go` | `openspec/specs/api-response-contract/spec.md` | ready |
| `database-schema-migrations` | 通过 Ent schema 和 Atlas 生成、审查、校验并部署服务内 SQL migration | `user-services/atlas.hcl`, `user-services/ent/migrate/main.go`, `user-services/migrations/`, `user-services/scripts/` | `openspec/specs/database-schema-migrations/spec.md` | ready |
| `go-toolchain-baseline` | 统一 Go workspace 与模块工具链版本，保证开发、测试和自动化环境一致 | `go.work`, `common/go.mod`, `user-services/go.mod`, `docs/DEVELOPMENT.md` | `openspec/specs/go-toolchain-baseline/spec.md` | ready |

## 2. Key Entry Points

- CLI：`user-services/cmd/main.go`
- Fx app：`user-services/internal/bootstrap/bootstrap.go`
- Router：`user-services/internal/router/router.go`
- Shared infrastructure：`common/infrastructure/config.go`, `common/infrastructure/logger.go`, `common/infrastructure/redis.go`, `common/infrastructure/postgres.go`
- API response helpers：`common/response/response.go`
- Database migrations：`user-services/atlas.hcl`, `user-services/migrations/`, `user-services/scripts/`

## 3. Cross-Capability Dependencies

- `user-profile-query` 依赖 `shared-infrastructure` 提供 Ent `user_db` client。
- `user-profile-query` 依赖 `api-response-contract` 输出统一成功和失败响应。
- `http-service-runtime` 依赖 `shared-infrastructure` 加载配置、初始化 Zap logger，并复用 Redis/PostgreSQL 单实例创建能力；用户服务运行时自身声明 `cache_redis`、`user_db`、`common_db`。
- `http-service-runtime` 依赖 `api-response-contract` 通过 recovery 中间件输出 panic 错误。
- `database-schema-migrations` 依赖 `shared-infrastructure` 的 PostgreSQL 命名实例配置约定，但迁移执行不得启动 Fx、Redis、HTTP server 或 Ent runtime client。
- 所有 Go capability 的实现、测试、Ent 生成和 Atlas helper 构建都依赖 `go-toolchain-baseline`。

## 4. Candidate Future Capabilities

这些能力目前没有足够实现基础，不应作为 ready 主规格：

- 用户创建、更新、禁用或删除。
- 认证、授权、会话或令牌管理。
- 支付服务或 `pay_db` 相关业务。
- 依赖级健康检查，例如 Redis/Postgres 健康状态聚合。

新增这些能力时，应先用 `/opsx:propose <change-name>` 创建变更，并在实现完成后补齐对应主规格。
