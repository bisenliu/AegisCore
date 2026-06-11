# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md`、`AGENTS.md` 和本 change 的 `proposal.md`、`design.md`，确认本次只整理 router 文件边界，不改变 HTTP 行为或 provider 组装。
- [x] 新增 `user-service/internal/router/health.go`。
- [x] 将 `healthStatusOK` 从 `user-service/internal/router/system.go` 迁移到 `health.go`。
- [x] 将 `HealthResponse` 从 `system.go` 迁移到 `health.go`，保持字段、JSON tag 和 Swagger example 不变。
- [x] 将 `/healthz` handler `healthz(serviceName string)` 从 `system.go` 迁移到 `health.go`，保持 HTTP 200 和响应体契约不变。
- [x] 将 `/healthz` Swagger annotations 随 handler 一起迁移到 `health.go`。
- [x] 将 `registerSystemRoutes` 替换为 `registerHealthRoutes(engine *gin.Engine, serviceName string)`，并只注册 `GET /healthz`。
- [x] 更新 `user-service/internal/router/router.go`，让 `RegisterUserServiceHTTPRoutes` 调用 `registerHealthRoutes`、`RegisterSwagger` 和 `registerV1Routes`。
- [x] 确认 `router.go` 只保留 `RouteParams`、`RegisterUserServiceHTTPRoutes` 和 `/api/v1` feature route grouping，不定义健康检查或 Swagger handler。
- [x] 确认 `user-service/internal/router/swagger.go` 继续承载 Swagger UI、`/docs`、`/api-docs` 和 Swagger enablement 逻辑。
- [x] 删除不再需要的 `user-service/internal/router/system.go`。
- [x] 运行 `gofmt -w` 格式化改动的 Go 文件。

## Tests

- [x] 新增或调整 `user-service/internal/router/health_test.go`，覆盖 `GET /healthz` 返回 HTTP 200。
- [x] 在健康检查测试中断言响应 JSON 仍包含 `status: "ok"`。
- [x] 在健康检查测试中断言响应 JSON 的 `service` 仍来自传入的 service name。
- [x] 确认 `user-service/internal/router/swagger_test.go` 仍覆盖 Swagger 默认启用/禁用、环境变量覆盖、UI 路径和 redirect。
- [x] 确认 `user-service/internal/providers/routes_test.go` 中 `/healthz` 和 Swagger route integration 测试继续通过。
- [x] 确认测试不依赖 PostgreSQL、Redis、Ent 或其他外部服务。

## Documentation

- [x] 更新 `AGENTS.md` Key Entry Points，增加健康检查路由文件 `user-service/internal/router/health.go`。
- [x] 更新 `AGENTS.md` 中 Gin router 路由定义说明，使 `router.go`、`health.go`、`swagger.go` 职责清晰。
- [x] 更新 `docs/ARCHITECTURE.md` HTTP Request Flow 或 Runtime Flow 中关于 router 的描述，说明健康检查位于 `health.go`，Swagger 位于 `swagger.go`，route graph 总装位于 `router.go`。
- [x] 扫描 `docs/DEVELOPMENT.md`，如存在 `system.go` 或健康检查文件位置的当前规则引用，则同步更新。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/` 工件。

## Verification

- [x] 在 `user-service/` 运行 `go test ./internal/router ./internal/providers`。
- [x] 运行 `rg -n "registerSystemRoutes|system.go" user-service/internal/router AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md`，确认无 active 旧引用。
- [x] 运行 `rg -n "registerHealthRoutes|func healthz|type HealthResponse|RegisterSwagger" user-service/internal/router`，确认健康和 Swagger 符号位于目标文件。
- [x] 检查 `git diff -- user-service/internal/router user-service/internal/providers AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/changes/split-user-service-router-health`，确认没有 `/healthz`、Swagger、`/api/v1`、认证中间件、配置、Ent schema 或 migration 的行为变更。
- [x] 如实现过程中运行了 Swagger 生成，检查 `user-service/docs/swagger.json`、`user-service/docs/swagger.yaml` 和 `user-service/docs/docs.go` 只有预期的非语义差异；否则不要更新生成文件。

## Review Notes

- [x] 确认没有新增 readiness 或依赖级 health check。
- [x] 确认没有把 router 逻辑迁移到 `internal/providers`。
- [x] 确认没有新增 Fx provider、middleware、外部依赖或配置 key。
- [x] 确认没有修改 feature route registration、auth middleware 或 response envelope。
- [x] 确认 `HealthResponse` 名称保持稳定，避免 Swagger definition 不必要漂移。
