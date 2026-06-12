# Tasks

## Implementation

- [x] 新增 `common/contract/errors/` 目录。
- [x] 新增 `common/contract/errors/errors.go`，迁移全局错误码主定义。
- [x] 在 `errors.go` 中迁移应用错误类型、wrap/unwrap、错误构造 helper 和 `FromError`。
- [x] 新增 `common/contract/errors/errors_test.go`，覆盖错误码数值、构造函数、wrap/unwrap 和 `FromError`。
- [x] 新增 `common/contract/pagination/` 目录。
- [x] 新增 `common/contract/pagination/cursor.go`，迁移 Cursor/Keyset 分页结构和 helper。
- [x] 新增 `common/contract/pagination/cursor_test.go`，覆盖分页大小规范化、分页元数据创建和 nil items 输出为空数组。
- [x] 更新 `common/contract/response/response.go`，让 `Envelope.Code` 使用 `contract/errors.Code`，`Fail` 使用 `contract/errors.FromError`。
- [x] 删除 `common/contract/response/error.go` 中对 `contract/errors` 的兼容 type alias、const alias 和 helper wrapper/alias。
- [x] 删除 `common/contract/response/pagination.go` 中对 `contract/pagination` 的兼容 const alias、type alias 和 helper wrapper/alias。
- [x] 调整 `common/contract/response/response_test.go`，保留 response envelope 兼容测试，并将错误和分页核心行为测试下沉到新包。
- [x] 迁移 `common/validation` 中非 HTTP response 层的错误码引用到 `common/contract/errors`。
- [x] 迁移 `common/http/ginvalidation` 中 validation error code 引用到 `common/contract/errors`。
- [x] 梳理 `common/http/middleware`：HTTP 输出继续使用 `response.Fail`，应用错误构造优先使用 `common/contract/errors`。
- [x] 迁移 `user-service/internal/features/user/transport/http/mapper.go` 中分页 payload 构造到 `common/contract/pagination`。
- [x] 迁移 `user-service/internal/features/user/api/doc.go` 的分页 DTO 类型到 `common/contract/pagination`。
- [x] 迁移适合直接引用错误契约的 user-service validation tests 或 helper 到 `common/contract/errors`。
- [x] 将剩余 `response.Code*` 测试断言迁移为直接引用 `common/contract/errors.Code*`。
- [x] 确认 `common/contract/response` 不再 re-export `errors` 或 `pagination`。
- [x] 运行 `gofmt -w` 处理修改过的 Go 文件。
- [x] 更新 `docs/ARCHITECTURE.md` 中 `common/contract` 的错误码、分页和 response 职责说明。
- [x] 必要时更新 `AGENTS.md` 中 `common/` Repository Shape 描述。
- [x] 将 `common/contract/errors/errors.go` 拆分为 `code.go`、`error.go`、`factory.go`、`convert.go`。
- [x] 将 `common/contract/pagination/cursor.go` 拆分为 `pagination.go`、`data.go`、`helper.go`。
- [x] 将 `common/contract/response/response.go` 调整为 `envelope.go`，并移除 Gin writer 实现。
- [x] 新增 `common/http/response/` 目录。
- [x] 新增 `common/http/response/writer.go`，承载 `OK`、`Created`、`NoContent`、`JSON`。
- [x] 新增 `common/http/response/error.go`，承载 `Fail`、`WriteError`、校验失败和常用错误响应输出。
- [x] 新增 `common/http/response/helper.go`，承载 envelope 构造和 HTTP status 辅助函数。
- [x] 新增 `common/http/response/response_test.go`，覆盖 Gin writer 输出行为。
- [x] 迁移 `common/http/ginvalidation` 的 writer 调用到 `common/http/response`。
- [x] 迁移 `common/http/middleware` 的 writer 调用到 `common/http/response`。
- [x] 迁移 user-service controller 的 writer 调用到 `common/http/response`。
- [x] 将 `common/http/response` 调用方改为不使用导入别名，直接使用 `response.*`。
- [x] 保留 Swagger 注释和 envelope 测试对 `common/contract/response.Envelope` 的引用。

## Verification

- [x] 在 `common/` 执行 `go test ./...`。
- [x] 在 `user-service/` 执行 `go test ./...`。
- [x] 如 Swagger 生成依赖可用，执行 `make swagger-generate` 并检查生成产物只反映契约包引用变化。
- [x] 检查分页响应 JSON 字段仍为 `items`、`pagination.page_size`、`pagination.next_cursor`、`pagination.has_next`。
- [x] 检查失败响应 JSON 字段仍为 `success`、`code`、`message`，字段级校验仍输出 `errors`。
- [x] 检查错误码数值未变化。
- [x] 运行 `rg -n "openspec|docs/opsx" .`，确认没有新增 OpenSpec/OPSX 工件。
- [x] 检查 `git diff`，确认没有业务逻辑、HTTP route、数据库 schema、migration 或 Redis key 的非预期变化。
- [x] 确认 `common/contract/response` 不再导入 Gin。
- [x] 确认 Gin writer 只位于 `common/http/response`。

## Review Notes

- [x] 确认 `common/contract/errors` 不导入 Gin、Ent、Redis、config、response 或 feature 包。
- [x] 确认 `common/contract/pagination` 不导入 Gin、response、errors、Ent、Redis 或 feature 包。
- [x] 确认 `common/contract/response` 只负责 HTTP response envelope DTO 和默认消息，不承载错误码、应用错误、分页或 Gin helper。
- [x] 确认调用方直接导入 `common/contract/errors` 和 `common/contract/pagination`，不再依赖 response re-export。
- [x] 确认文档没有把 `/opsx:*` 或 `openspec` 描述为当前工作流。
- [x] 确认 `common/http/response` 只做 HTTP 输出适配，不承载错误码、分页或 envelope 主定义。
