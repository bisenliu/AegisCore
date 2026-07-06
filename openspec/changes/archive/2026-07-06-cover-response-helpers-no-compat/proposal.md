## Why

`common/http/response` 的多个薄 wrapper 当前缺少直接测试覆盖，容易在重构中漂移统一 response envelope、错误码、message 或 HTTP status。现在补齐这些测试，可以把共享 HTTP 响应契约固定在当前实现上，并避免旧 envelope 或旧 helper alias 的兼容路径回流。

## What Changes

- 为 `Created`、`NoContent`、`ValidationFailed`、`Unauthenticated`、`Forbidden`、`Conflict`、`NotFound` 等 HTTP response helper 增加直接单元测试。
- 测试锁定当前统一响应 envelope、错误码、message、data 和 HTTP status；`204 No Content` 必须保持无 body。
- 新增测试遵循当前测试断言规范，优先使用语义化 `require`/`assert`，不新增旧式手写失败判断或兼容断言 helper。
- 不修改 `common/contract/errors` 错误码定义，不修改 feature controller 调用方式，不新增旧 envelope、旧错误码、旧 status 或旧 helper alias 兼容分支。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 补充共享 HTTP response helper 的稳定行为约束，要求薄 wrapper 的测试覆盖直接锁定统一 envelope、错误码和 HTTP status。

## Impact

- 影响代码路径：`common/http/response/error.go`、`common/http/response/writer.go`、`common/http/response/helper.go` 及同包测试。
- 不影响 HTTP API 路由、OpenAPI 生成物、数据库 schema、部署资产或外部依赖。
- 验证重点为 `go test -cover ./common/http/response`、`go tool cover -func` 中相关 wrapper 覆盖率，以及 `openspec validate cover-response-helpers-no-compat`。
