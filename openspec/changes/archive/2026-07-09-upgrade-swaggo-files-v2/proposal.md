## Why

`user-service` 当前使用 `github.com/swaggo/files v1.0.1` 提供 OpenAPI UI 静态资源，依赖版本已经落后于上游 v2 模块路径与初始化方式。升级到 v2 可以减少旧模块路径和旧 handler 用法的维护成本，并让运行时 OpenAPI UI 路由直接使用 v2 的标准资源入口。

## What Changes

- **BREAKING** 将 `user-service` 的 `github.com/swaggo/files` 依赖从 v1 升级为 v2 模块路径和版本，不保留 v1 import、旧 handler 包装或兼容分支。
- 调整 `user-service/internal/router/openapi.go` 中 OpenAPI UI 静态资源的导入、初始化和路由挂载方式，全面使用 v2 暴露的 embedded `fs.FS`。
- 检查并更新受影响的 `go.mod`、`go.sum`、OpenAPI 路由测试、生成验证和必要的初始化逻辑，确保 `/openapi/*any`、`/openapi.json`、`/docs`、`/api-docs` 的现有运行时语义保持清晰。
- 不新增旧路径、旧 handler、旧静态资源或双写兼容代码；测试只覆盖升级后的当前行为。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `runtime-observability`: OpenAPI UI 运行时路由必须使用 `github.com/swaggo/files/v2` 的 embedded `fs.FS` 提供 Swagger UI 静态资源，并且不得保留 v1 handler 或兼容分支。
- `delivery-operations`: user-service 依赖、测试和验证流程必须覆盖 `github.com/swaggo/files/v2` 升级后的模块解析、构建、OpenAPI 生成和 drift 检查。

## Impact

- 受影响代码：`user-service/internal/router/openapi.go`、相关 router 测试、`user-service/go.mod`、`user-service/go.sum`。
- 受影响依赖：`github.com/swaggo/files` 从 v1 模块升级为 `github.com/swaggo/files/v2`，并移除依赖 v1 `webdav.Handler` 的 `github.com/swaggo/gin-swagger` 包装。
- 受影响运行时：OpenAPI JSON、Swagger UI 和 docs redirect 仍在 `/api/v1` 外注册，并继续由 `OPENAPI_ENABLED` 与环境默认值控制。
- 受影响验证：需要运行 user-service 相关测试、OpenAPI 生成检查、架构 lint，并在最终完整验证中确认没有生成物 drift。
- 不影响数据库 schema、Ent 生成代码、Atlas migration、业务 API 响应 envelope、RBAC policy、metrics/tracing 指标契约或部署资产。
