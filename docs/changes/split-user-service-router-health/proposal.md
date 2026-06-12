# Split user-service router health

## What

将 `user-service/internal/router` 中的系统路由整理为更清晰的文件边界：

```text
user-service/internal/router/
  router.go
  health.go
  swagger.go
```

目标是让 `router.go` 只保留 Gin engine/router 的基础路由总装和 `/api/v1` feature route grouping；健康检查路由迁移到 `health.go`；Swagger UI 和文档重定向继续保留在 `swagger.go`。

当前健康检查实现位于 `system.go`，其中包含 `/healthz` route registration、`HealthResponse` DTO 和 handler。该变更会将这些内容移动到 `health.go`，并让 `RegisterUserServiceHTTPRoutes` 调用健康路由注册函数。

## Why

`router` 包已经形成了三类不同责任：

- 基础路由总装：系统路由、Swagger 和 `/api/v1` feature route grouping。
- 健康检查：`/healthz` 最小存活响应。
- Swagger：Swagger UI、`/docs` 和 `/api-docs` redirect，以及环境变量开关。

把健康检查放入明确命名的 `health.go` 后，`router.go` 会更专注于 route graph 组合，Swagger 逻辑也继续保持独立。这个拆分是小范围结构整理，不改变 HTTP API、认证、feature 路由或 provider 组装边界。

## Scope

包括：

- 新增 `user-service/internal/router/health.go`。
- 将 `HealthResponse`、`healthStatusOK`、`healthz` handler 和健康检查 route registration 从当前系统路由文件迁移到 `health.go`。
- 删除或清空不再需要的 `user-service/internal/router/system.go`。
- 保留 `user-service/internal/router/swagger.go` 作为 Swagger UI、文档重定向和 Swagger 开关的唯一实现位置。
- 保持 `user-service/internal/router/router.go` 只负责 `RegisterUserServiceHTTPRoutes`、`RouteParams` 和 `/api/v1` route group 组装。
- 增加或调整健康检查测试，覆盖 `/healthz` 状态码和响应体。
- 保留现有 Swagger 测试，并根据拆分后文件/函数命名做必要更新。
- 更新 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 中 router 入口和职责说明。

不包括：

- 不新增依赖级 readiness 检查。
- 不增加数据库、Redis、Ent 或外部服务健康探测。
- 不改变 `/healthz` 的路径、HTTP status、JSON 字段或现有响应契约，除非已有测试明确要求。
- 不改变 Swagger 的启用规则、route path 或 redirect 行为。
- 不改变 `/api/v1`、认证中间件、用户 feature 或认证 feature 的路由行为。
- 不移动 service-level Fx provider；它们继续位于 `user-service/internal/providers`。
- 不改变 Ent schema、generated code、Atlas migration、配置 key 或部署资产。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `user-service/internal/router/health.go` 承载 `/healthz` 相关 DTO、handler 和 route registration。
- `user-service/internal/router/swagger.go` 继续承载 Swagger UI、`/docs`、`/api-docs` 和 Swagger 开关逻辑。
- `user-service/internal/router/router.go` 只保留 route params、顶层 route registration 和 `/api/v1` feature route grouping。
- `system.go` 不再作为系统路由兜底文件存在；如保留，必须不承载健康检查或 Swagger 逻辑。
- `/healthz` 仍返回 HTTP 200，响应体仍包含现有 `status` 与 `service` 字段语义。
- Swagger UI 和 redirect 测试继续通过。
- 健康检查测试覆盖拆分后的 route registration 和响应契约。
- `AGENTS.md` 与 `docs/ARCHITECTURE.md` 中 router 文件职责说明同步更新。
- 在 `user-service/` 下运行相关 router/provider 测试通过。
