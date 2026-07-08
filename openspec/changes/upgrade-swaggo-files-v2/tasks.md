## 1. 依赖与影响面确认

- [x] 1.1 确认 `user-service` 中所有 `github.com/swaggo/files`、`swaggerFiles` 和 `ginSwagger` 使用点，记录只需修改 `user-service/internal/router/openapi.go` 与模块依赖。
- [x] 1.2 查阅 `github.com/swaggo/files/v2` 当前标准用法，确认与 `github.com/swaggo/gin-swagger` 的 handler 调用方式。
- [x] 1.3 审查 `user-service/go.mod` 和 `user-service/go.sum` 当前状态，确认不会保留 v1 依赖或双版本依赖。

## 2. 代码实现

- [x] 2.1 将 `user-service/go.mod` 中 `github.com/swaggo/files` 升级为 `github.com/swaggo/files/v2`，并更新 `go.sum`。
- [x] 2.2 修改 `user-service/internal/router/openapi.go` 的 import 和 Swagger UI handler 初始化，全面使用 v2 标准用法。
- [x] 2.3 确认生产代码中不存在 `github.com/swaggo/files` v1 import、旧 handler fallback、版本探测分支或 v1/v2 双写兼容代码。
- [x] 2.4 确认 OpenAPI JSON、Swagger UI、`/docs` 和 `/api-docs` redirect 仍由 `RegisterOpenAPI` 在 `/api/v1` 外注册，并继续受 `OPENAPI_ENABLED` 与环境默认值控制。

## 3. 测试与生成物

- [x] 3.1 更新或补充 `user-service/internal/router` 测试，覆盖 `/openapi.json`、`/openapi/index.html` 或等价 Swagger UI 路由、docs redirect 和生产环境默认关闭行为。
- [x] 3.2 运行 `go test ./user-service/internal/router` 并确认通过。
- [x] 3.3 运行 `make user-service-openapi-generate`，确认 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml` 可生成且无非预期 drift。
- [x] 3.4 审查 `git diff`，确认依赖、代码、测试和生成物变更均为本 change 预期范围。

## 4. OPSX 与完整验证

- [x] 4.1 运行 `make user-service-architecture-lint`，确认 OpenSpec artifacts 与架构边界检查通过。
- [x] 4.2 暂存本次预期代码、依赖、OpenSpec artifacts 和必要生成物变更。
- [x] 4.3 运行 `make lint` 并确认通过。
- [x] 4.4 运行 `make verify` 并确认通过，最终 `git diff --exit-code` 不报告未纳入暂存区的预期变更。
- [x] 4.5 若任一验证失败，修复问题并重新执行对应验证后再勾选任务。
