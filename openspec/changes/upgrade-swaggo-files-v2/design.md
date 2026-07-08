## Context

`user-service/internal/router/openapi.go` 当前通过 `github.com/swaggo/files` v1 的 `swaggerFiles.Handler` 与 `github.com/swaggo/gin-swagger` 注册 `/openapi/*any`。v2 模块 `github.com/swaggo/files/v2` 暴露 embedded `fs.FS`，而已发布的 `gin-swagger v1.6.1` 仍接收 v1 风格的 `*webdav.Handler`，因此继续使用 `gin-swagger` 会把实现绑定回旧 handler 模型。

本 change 只涉及 OpenAPI UI 静态资源依赖和运行时路由挂载方式，不改变 OpenAPI 3 文档内容生成、HTTP API 契约、认证授权、数据库 schema、Ent 生成物或部署资产。OpenAPI JSON 仍由 `user-service/docs` embed 生成物提供，Swagger UI 仍由 `RegisterOpenAPI` 在 `/api/v1` 外注册，并继续受 `OPENAPI_ENABLED` 与环境默认值控制。

## Goals / Non-Goals

**Goals:**

- 将 `user-service` 从 `github.com/swaggo/files` v1 迁移到 `github.com/swaggo/files/v2`。
- 使用 v2 embedded `fs.FS` 直接提供 Swagger UI 静态资源，删除 v1 import、旧 handler、`gin-swagger` 包装或兼容分支。
- 保持 `/openapi.json`、`/openapi/*any`、`/docs` 和 `/api-docs` 的当前路由职责与授权边界不变。
- 通过 Go 测试、OpenAPI 生成、架构 lint 和完整验证确认依赖升级没有引入 drift。

**Non-Goals:**

- 不修改业务 API、response envelope、RBAC 授权策略、健康检查、metrics、tracing 或 pprof 路由。
- 不修改 `common/http/openapi` 的 Swagger 2 到 OpenAPI 3 转换契约。
- 不修改 `tools/openapi-convert` CLI 参数、输出文件集合或服务脚本传入参数。
- 不引入 v1/v2 双写、旧资源 fallback、旧 import alias 或测试专用生产分支。
- 不修改数据库 schema、Ent 生成代码、Atlas migration、Docker、Compose、Kubernetes、Helm 或观测 dashboard。

## Decisions

### Decision 1: 直接迁移到 v2 模块路径

实施时将 `user-service/go.mod` 中的 `github.com/swaggo/files` 替换为 `github.com/swaggo/files/v2`，并通过 `go mod tidy` 或等价模块解析更新 `go.sum`。

备选方案是保留 v1 依赖并只升级 patch/minor 版本。该方案不满足“升级到 v2”和“不保留向后兼容代码”的要求，因此拒绝。

### Decision 2: OpenAPI UI 静态资源服务仍归属 router 层

OpenAPI UI 的 v2 静态资源服务继续放在 `user-service/internal/router/openapi.go`，因为该文件已经拥有 OpenAPI JSON、UI 和 docs redirect 的路由注册职责。实现直接使用 `github.com/swaggo/files/v2` 的 `FS`、Go 标准库 `http.FileServerFS` 和少量路由内重写逻辑，将 `swagger-initializer.js` 中的默认 Petstore spec URL 替换为 `/openapi.json`。不会把 v2 静态资源服务下沉到 `common/http/openapi`，也不会放入 feature 包或 provider 包。

备选方案是在 `common/http/openapi` 增加 Swagger UI helper。该方案会把服务专属路由路径、Gin handler 和 UI 初始化细节推入跨服务共享模块，不符合 common 只承载通用转换和渲染 helper 的边界，因此拒绝。

### Decision 3: 移除 gin-swagger 包装且不增加兼容别名或 fallback

实施时只保留升级后的 v2 import 和静态资源服务逻辑，测试也只验证当前 v2 行为。`/openapi/*any`、`/docs` 和 `/api-docs` 是当前稳定路由，不是 v1 兼容路径，因此保留；不会新增旧 UI 静态路径、旧 handler fallback、`gin-swagger` wrapper 或版本探测分支。

备选方案是在运行时按依赖或配置选择 v1/v2 handler。该方案增加无必要复杂度，并违反本 change 的无兼容代码约束，因此拒绝。

### Decision 4: 验证范围聚焦 user-service OpenAPI 路由和交付链路

验证需要覆盖 `user-service/internal/router` 测试、`make user-service-openapi-generate`、`make user-service-architecture-lint`、`make lint` 和 `make verify`。如果 v2 依赖更新不改变 OpenAPI JSON/YAML 内容，生成物 diff 应为空；如果工具链产生格式化或生成物更新，必须审查并纳入本 change。

备选方案是只运行 `go test`。该方案无法覆盖 OpenAPI 生成物 drift、OPSX 文档语言约束和完整交付链路，因此不足。

## Risks / Trade-offs

- `github.com/swaggo/files/v2` 的 embedded `fs.FS` 服务方式与旧 `gin-swagger` wrapper 行为存在差异 -> 使用 `http.FileServerFS` 服务静态资源，并通过 router 测试验证 index、initializer、CSS、JSON 和 redirect 行为。
- v2 Swagger UI 静态资源路径或默认 index 行为变化 -> 保持 `/openapi/*any` 注册路径不变，并补充或更新 router 测试验证 `/openapi/index.html` 可访问以及 redirect 目标不变。
- 依赖升级引入 `go.sum` 或间接依赖变化 -> 审查 `go.mod`、`go.sum` diff，确认只包含 v2 迁移所需变更。
- 完整验证的 `git diff --exit-code` 因预期变更未暂存失败 -> 在最终执行 `make lint` 和 `make verify` 前先暂存本次预期代码、文档和生成物变更。

## Migration Plan

1. 更新 `user-service` 模块依赖为 `github.com/swaggo/files/v2`，移除 v1 依赖。
2. 调整 `user-service/internal/router/openapi.go` 的 import 和 Swagger UI 静态资源服务逻辑，使用 v2 embedded `fs.FS`。
3. 更新或补充 router 测试，验证 OpenAPI JSON、Swagger UI、docs redirect 和生产环境默认关闭语义。
4. 运行模块整理和相关测试，审查 `go.mod`、`go.sum` 和生成物 diff。
5. 运行 `make user-service-openapi-generate` 和 `make user-service-architecture-lint`。
6. 暂存本次预期变更后运行 `make lint` 和 `make verify`。

回滚方式：将 `user-service` 依赖和 `openapi.go` handler 调用恢复到变更前提交，并重新运行相关测试和生成检查。由于不涉及数据库和外部 API，回滚不需要数据迁移。

## Open Questions

无。实施时若上游 v2 API 与预期不一致，以 `github.com/swaggo/files/v2` 当前发布版本文档和编译结果为准，同时保持不引入 v1 兼容分支的约束。
