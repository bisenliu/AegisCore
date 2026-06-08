## Why

当前仓库已有较多单元测试和切片测试，但 Repository、完整 HTTP 中间件链、以及 Redis token version 缓存与认证路径之间缺少贯通式集成验证。随着 `user-profile-query`、`user-authentication`、`user-session-control`、`http-service-runtime`、`shared-infrastructure` 和 `api-response-contract` 的边界逐步稳定，需要用集成测试锁定跨层行为，降低后续改动引入回归的风险。

## What Changes

- 新增一组用户服务集成测试要求，覆盖 Repository 数据访问边界、HTTP 端到端请求响应、认证与 trace-id/logging/recovery 中间件链、统一响应信封，以及 Redis token version 缓存的真实交互路径。
- Repository 集成测试应优先复用 Ent SQLite 测试库进行快速验证；仅在需要验证 PostgreSQL 特有语义时引入 `testcontainers-go` 启动真实 PostgreSQL。
- HTTP 端到端测试应使用 `httptest` 通过真实 Gin engine 和路由注册执行请求，并覆盖 401、403、404、500 等错误信封。
- Redis 集成测试应使用 `miniredis` 验证 token version cache miss、backfill、TTL、失效、以及旧 token 被认证中间件拒绝的贯通行为。
- 不修改对外 API、错误码、配置格式、数据库模型或运行时启动方式。

## Capabilities

### New Capabilities
- `integration-test-coverage`: 定义用户服务跨 Repository、HTTP 中间件链和 Redis token version 缓存路径的集成测试覆盖要求。

### Modified Capabilities
- 无。

## Impact

- 受影响代码主要位于 `user-services/internal/repository/postgres/`、`user-services/internal/repository/redis/`、`user-services/internal/bootstrap/`、`user-services/internal/router/`、`user-services/internal/controller/`、`user-services/internal/service/` 和 `common/http/middleware/` 的测试文件。
- 可能新增测试依赖：`testcontainers-go` 仅在必须验证 PostgreSQL 特有行为时使用；Redis 集成路径继续优先使用已有 `miniredis` 依赖。
- 不影响 HTTP API 兼容性、响应信封结构、错误码语义、配置键、数据库 schema 或 Ent 生成代码。
- 验证范围依赖 `go-toolchain-baseline`，实现完成后应分别在 `common/` 和 `user-services/` 运行 `go test ./...`。
