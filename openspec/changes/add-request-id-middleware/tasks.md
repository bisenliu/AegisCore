## 1. Common middleware

- [x] 1.1 在 `common/http/middleware` 新增 request ID middleware，支持读取、校验、生成和响应头回写 `X-Request-ID`。
- [x] 1.2 在 `common/http/middleware` 提供 request ID context helper，使后续 middleware 和 handler 可从 `context.Context` 读取最终 `request_id`。
- [x] 1.3 扩展 `common/http/middleware` 请求日志字段，使 access log 在存在 request ID 时输出稳定 `request_id` 字段。
- [x] 1.4 为 common middleware 增加单元测试，覆盖合法 header 透传、缺失 header 生成、空白/超长/控制字符 header 拒绝、响应头回写和日志字段输出。

## 2. User-service wiring

- [x] 2.1 在 `user-service/internal/providers/gin.go` 的全局 Gin middleware 链中安装 request ID middleware，并保证其位于 `RequestLoggerWithOptions` 之前。
- [x] 2.2 增加 `user-service/internal/providers/gin_test.go` 覆盖服务级 `X-Request-ID` 透传、缺失生成和与 `traceparent` 并存场景。

## 3. Verification

- [x] 3.1 运行 `go test ./...` 于 `common` 模块，确认共享 middleware、logger 和 metrics 相关测试通过。
- [x] 3.2 运行 `go test ./...` 于 `user-service` 模块，确认 Gin provider 和路由相关测试通过。
- [x] 3.3 运行 `make user-service-architecture-lint`，确认新增实现没有违反 common、feature、shared 或 provider 边界。
- [x] 3.4 检查 `git diff --exit-code -- user-service/docs/openapi.json user-service/docs/openapi.yaml user-service/docs/openapi.go deployments`，确认本变更未产生 OpenAPI 或部署观测资产 drift。
