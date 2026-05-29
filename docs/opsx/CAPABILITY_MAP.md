# Capability Map

本地图将长期稳定能力映射到代码模块和 OpenSpec 主规格。提出或实现变更前，先定位相关 capability。

## 1. Capabilities

| Capability | 业务说明 | 主要代码位置 | 主规格 | 状态 |
|---|---|---|---|---|
| `user-profile-query` | 通过用户 ID 查询用户资料并返回统一 API 响应 | `user-services/internal/router/router.go`, `user-services/internal/controller/user_controller.go`, `user-services/internal/service/user_service.go`, `user-services/internal/repository/user_repository.go`, `user-services/ent/schema/user.go` | `openspec/specs/user-profile-query/spec.md` | ready |
| `http-service-runtime` | 通过 CLI 启动 HTTP 服务，注册中间件和路由，并支持优雅关闭 | `user-services/cmd/main.go`, `user-services/internal/bootstrap/bootstrap.go`, `user-services/internal/router/router.go` | `openspec/specs/http-service-runtime/spec.md` | ready |
| `shared-infrastructure` | 加载配置并通过 Fx 提供日志、Redis、Postgres 和 Ent clients | `common/config/`, `common/infrastructure/`, `user-services/internal/entclient/provider.go` | `openspec/specs/shared-infrastructure/spec.md` | ready |
| `api-response-contract` | 统一 HTTP 成功/失败信封、错误码和应用错误映射 | `common/response/`, `common/middleware/recovery.go`, `user-services/internal/controller/user_controller.go` | `openspec/specs/api-response-contract/spec.md` | ready |
| `database-schema-migrations` | 通过 Ent schema 和 Atlas 生成、审查、校验并部署服务内 SQL migration | `user-services/atlas.hcl`, `user-services/ent/migrate/main.go`, `user-services/migrations/`, `user-services/scripts/` | `openspec/specs/database-schema-migrations/spec.md` | ready |

## 2. Key Entry Points

- CLI：`user-services/cmd/main.go`
- Fx app：`user-services/internal/bootstrap/bootstrap.go`
- Router：`user-services/internal/router/router.go`
- Shared infrastructure：`common/infrastructure/module.go`
- API response helpers：`common/response/response.go`

## 3. Cross-Capability Dependencies

- `user-profile-query` 依赖 `shared-infrastructure` 提供 Ent `user_db` client。
- `user-profile-query` 依赖 `api-response-contract` 输出统一成功和失败响应。
- `http-service-runtime` 依赖 `shared-infrastructure` 完成配置、日志、Redis 和 Postgres 初始化。
- `http-service-runtime` 依赖 `api-response-contract` 通过 recovery 中间件输出 panic 错误。
- `database-schema-migrations` 依赖 `shared-infrastructure` 的 PostgreSQL 命名实例配置约定，但迁移执行不得启动 Fx、Redis、HTTP server 或 Ent runtime client。

## 4. Candidate Future Capabilities

这些能力目前没有足够实现基础，不应作为 ready 主规格：

- 用户创建、更新、禁用或删除。
- 认证、授权、会话或令牌管理。
- 支付服务或 `pay_db` 相关业务。
- 依赖级健康检查，例如 Redis/Postgres 健康状态聚合。

新增这些能力时，应先用 `/opsx:propose <change-name>` 创建变更，并在实现完成后补齐对应主规格。
