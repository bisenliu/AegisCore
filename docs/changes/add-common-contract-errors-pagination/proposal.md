# Add common contract errors pagination

## What

新增稳定的全局契约包 `common/contract/errors` 和 `common/contract/pagination`，将当前集中在 `common/contract/response` 中的错误码和 keyset 分页契约拆分到更清晰的位置。

包括：

- 新增 `common/contract/errors/`，承载全局应用错误码、可渲染应用错误类型和错误构造/转换 helper。
- 新增 `common/contract/pagination/`，承载 Cursor/Keyset 分页结构、分页大小默认值/上限和分页 helper。
- 梳理 `common/contract/response` 与新契约包的关系，使 response 只负责 HTTP 响应信封 DTO 和默认消息。
- 新增 `common/http/response`，承载 Gin HTTP 响应输出 helper。
- 移除 `response` 包对错误码、应用错误和分页契约的兼容导出，让调用方直接导入 `errors` 与 `pagination`。
- 为新包补充单元测试，并更新架构、开发或测试文档中的契约说明。

本变更不改变任何 HTTP 响应 JSON 字段、错误码数值、错误码语义、默认错误消息或分页响应字段。

## Why

`common/contract/response` 原本同时承担三类职责：

- HTTP 响应信封与 Gin 输出 helper。
- 全局应用错误码和错误转换。
- Cursor/Keyset 分页数据结构和分页大小规范化。

错误码、分页结构和响应信封是跨服务稳定契约，不应该依赖 Gin。把错误和分页拆到 `common/contract/errors` 与 `common/contract/pagination`，再把 Gin 输出函数移到 `common/http/response` 后，业务 app、validation、middleware、future services 和文档可以引用更精确的契约位置，避免把非 HTTP 输出逻辑绑定到 response writer。

本次迁移直接收敛到新包边界，避免继续通过 `response` 暴露错误码、应用错误或分页类型，从而让调用方依赖关系更明确。

## Scope

包括：

- 新增 `common/contract/errors/`。
- 新增 `common/contract/pagination/`。
- 将错误码 `Code`、`CodeOK`、`CodeBadRequest`、`CodeValidationFailed`、`CodeUnauthenticated`、`CodeTokenInvalid`、`CodeTokenExpired`、`CodeTokenRevoked`、`CodeMFARequired`、`CodeUserAccountLocked`、`CodeForbidden`、`CodeConflict`、`CodeNotFound`、`CodeInternalError` 移到 `errors` 包作为主定义。
- 将应用错误类型 `Error`、`NewError`、`Wrap`、错误构造 helper 和 `FromError` 移到 `errors` 包作为主实现。
- 将 `DefaultPageSize`、`MaxPageSize`、`Pagination`、`PaginatedData`、`NormalizePageSize`、`NewPagination`、`NewPaginatedData` 移到 `pagination` 包作为主定义。
- 将 `common/contract/response` 收敛为 `Envelope` 和默认消息，不再提供错误码、应用错误或分页 type/const alias。
- 新增 `common/http/response`，提供 `OK`、`Created`、`NoContent`、`JSON`、`Fail`、`ValidationFailed`、`WriteError` 等 Gin 输出 helper。
- 一次受控迁移中把现有 writer 调用迁到 `common/http/response`。
- 迁移 `common/validation`、`common/http` 和 `user-service` 中适合直接引用新契约包的调用点。
- 保持 `common/contract/response.Envelope` 的 JSON shape 不变。
- 更新 Swagger-facing DTO 引用，确保生成的响应字段仍为 `items` 和 `pagination`，分页字段仍为 `page_size`、`next_cursor`、`has_next`。
- 补充或拆分单元测试：`common/contract/errors`、`common/contract/pagination` 和 `common/contract/response`。
- 更新 `docs/ARCHITECTURE.md`，必要时同步 `AGENTS.md` 的 common contract 说明。

不包括：

- 不改变 HTTP 响应 JSON 字段。
- 不改变现有错误码数值或语义。
- 不改变默认公开错误消息，例如 `internal server error`。
- 不改变分页字段名、默认分页大小或最大分页大小。
- 不引入 offset/page/total/total_pages 分页响应字段。
- 不改变 user-service 业务逻辑、路由、认证流程、数据库 schema、migration 或 Redis key。
- 不新增 `openspec/` 或 `docs/opsx/`。

## Acceptance Criteria

- 存在 `common/contract/errors/code.go`、`error.go`、`factory.go`、`convert.go`，并有对应单元测试覆盖错误码数值、错误构造、wrap/unwrap 和 `FromError` 行为。
- 存在 `common/contract/pagination/pagination.go`、`data.go`、`helper.go`，并有对应单元测试覆盖分页大小规范化、分页元数据创建和 nil items 输出为空数组。
- `common/contract/response` 不再导入 Gin，只保留响应 DTO 和默认消息，不 re-export `errors` 或 `pagination`。
- `common/http/response` 提供 Gin 输出 helper，现有 writer 调用已迁移。
- HTTP 成功/失败 envelope 的 JSON 字段保持兼容：`success`、`code`、`message`、`data`、`errors`。
- 分页响应 JSON 字段保持兼容：`items`、`pagination.page_size`、`pagination.next_cursor`、`pagination.has_next`。
- `common/` 中 `go test ./...` 通过。
- `user-service/` 中 `go test ./...` 通过。
- 文档说明 `common/contract/errors` 和 `common/contract/pagination` 是稳定契约位置，`response` 负责 HTTP 响应信封 DTO。
