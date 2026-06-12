# Design

## Overview

本变更把 `common/contract/response` 中的全局错误契约和分页契约拆到专用包：

- `common/contract/errors`：应用层响应码、应用错误类型、错误构造和任意错误到应用错误的转换。
- `common/contract/pagination`：Cursor/Keyset 分页 DTO、分页大小边界和分页数据包装 helper。
- `common/contract/response`：HTTP 响应信封 DTO 和默认响应消息。
- `common/http/response`：Gin HTTP 响应输出适配层。

拆分后，`response.Envelope` 的 `Code` 字段使用 `errors.Code`，分页响应 payload 使用 `pagination.PaginatedData[T]`。Gin 输出函数移到 `common/http/response`，由该包调用 `errors.FromError` 映射错误并写入 `contract/response.Envelope`。调用方直接导入 `common/contract/errors` 和 `common/contract/pagination`，`common/contract/response` 不 re-export 错误码、应用错误或分页类型。

## Package Layout

目标目录：

```text
common/contract/
  errors/
    code.go
    error.go
    factory.go
    convert.go
    errors_test.go
  pagination/
    pagination.go
    data.go
    helper.go
    pagination_test.go
  response/
    envelope.go
    message.go
    response_test.go
common/http/
  response/
    writer.go
    error.go
    helper.go
    response_test.go
```

`contract/response` 不导入 Gin，不提供 writer，也不 re-export `errors` 或 `pagination`。`common/http/response` 是唯一提供 Gin response writer 的共享包。

## Errors Contract

`common/contract/errors/code.go` 定义：

```go
package errors

type Code int

const (
    CodeOK Code = 0
    CodeBadRequest Code = 10000
    CodeValidationFailed Code = 10001
    CodeUnauthenticated Code = 20000
    CodeTokenInvalid Code = 20001
    CodeTokenExpired Code = 20002
    CodeTokenRevoked Code = 20003
    CodeMFARequired Code = 20004
    CodeUserAccountLocked Code = 20005
    CodeForbidden Code = 30000
    CodeConflict Code = 40000
    CodeNotFound Code = 50000
    CodeInternalError Code = 90000
)
```

同时迁移当前应用错误相关行为：

- `type Error struct { Code Code; Message string; HTTPStatus int; Cause error }`
- `func (e *Error) Error() string`
- `func (e *Error) Unwrap() error`
- `func NewError(code Code, message string, status int) *Error`
- `func Wrap(err error, code Code, message string, status int) *Error`
- `BadRequestError`
- `ValidationFailedError`
- `UnauthenticatedError`
- `TokenInvalidError`
- `TokenExpiredError`
- `ForbiddenError`
- `ConflictError`
- `NotFoundError`
- `WrapInternal`
- `InternalError`
- `FromError`

`errors` 包不应导入 Gin、Ent、Redis、HTTP binder 或 service/feature 包。它可以导入标准库 `errors`、`fmt`、`net/http`。为避免包名冲突，文件内标准库 errors import 使用别名，例如 `stderrors "errors"`。

### Message Ownership

`errors` 包拥有默认内部错误公开消息 `MessageInternalError = "internal server error"`，`errors.InternalError` 和 `errors.WrapInternal` 使用该消息。`response.MessageInternalError` 继续等于该值，供 HTTP envelope 默认消息断言和 writer 使用。`MessageOK`、`MessageCreated` 和 `MessageAuthInvalid` 暂留在 `response`，因为当前结构仍保留 `response/message.go`；如果后续要把默认消息也移出 `response`，应另起更小变更。

## Pagination Contract

`common/contract/pagination/pagination.go` 和 `data.go` 定义当前 keyset 分页契约：

```go
package pagination

const (
    DefaultPageSize = 10
    MaxPageSize = 100
)

type Pagination struct {
    PageSize   int    `json:"page_size" example:"50"`
    NextCursor string `json:"next_cursor,omitempty" example:"0190c8d2-8d8a-7a01-9f43-0f91fb4e2b7c"`
    HasNext    bool   `json:"has_next" example:"true"`
}

type PaginatedData[T any] struct {
    Items      []T        `json:"items"`
    Pagination Pagination `json:"pagination"`
}
```

函数行为保持不变：

- `NormalizePageSize(pageSize int) int`：小于 1 使用默认值，大于最大值时裁剪。
- `NewPagination(pageSize int, nextCursor string, hasNext bool) Pagination`：统一规范化 `PageSize`。
- `NewPaginatedData[T any](items []T, pagination Pagination) PaginatedData[T]`：nil items 转为空 slice，保证 JSON 输出数组而非 null。

`pagination` 包不依赖 Gin、response、errors、Ent、Redis 或 feature 包。

## Response And HTTP Writer Boundary

`common/contract/response` 作为响应 DTO 契约入口：

- `Envelope` 保持同名同字段，`Code` 字段类型改为 `contract/errors.Code`。
- `MessageOK`、`MessageCreated`、`MessageInternalError`、`MessageAuthInvalid` 保持兼容。
- 不导出 `Code`、`Error`、错误构造 helper、`Pagination` 或 `PaginatedData`。
- 不导入 Gin，不提供 `OK`、`Created`、`Fail`、`BadRequest` 等 writer。

`common/http/response` 提供 Gin writer：

- `OK(c, data)`
- `Created(c, data)`
- `NoContent(c)`
- `JSON(c, status, payload)`
- `Fail(c, err)`
- `WriteError(c, *errors.Error)`
- `BadRequest`、`ValidationFailed`、`ValidationFailedWithErrors`
- `Unauthenticated`、`TokenInvalid`、`TokenExpired`、`Forbidden`、`Conflict`、`NotFound`

该包可导入 Gin、`contract/errors` 和 `contract/response`，但不承载错误码或 envelope 主定义。

## Caller Migration

迁移顺序建议：

1. 新增 `errors` 和 `pagination` 包及测试。
2. 将 `contract/response.Envelope.Code` 改为使用 `contract/errors.Code`，但不保留错误或分页 re-export。
3. 新增 `common/http/response` 并迁移 Gin writer 函数。
4. 迁移 common 内部非 response 层调用方：
   - `common/validation` 使用 `contract/errors.Code`。
   - `common/http/ginvalidation` 使用 `contract/errors.CodeBadRequest` 等错误码。
   - `common/http/middleware` 调用 `common/http/response` 输出 HTTP envelope，构造应用错误时优先使用 `contract/errors`。
5. 迁移 user-service 中适合直接引用新包的调用点：
   - `features/*/transport/http/validation_test.go` 中的 `FromError` 与错误码断言。
   - user list mapper 使用 `pagination.NewPaginatedData` 和 `pagination.NewPagination`。
   - user API doc DTO 的分页类型使用 `pagination.Pagination`。
6. Controller 使用 `common/http/response` 写响应；Swagger 注释和 envelope 测试仍可引用 `contract/response.Envelope`。
7. 所有错误码断言直接引用 `contract/errors.Code*`，不再通过 `response.Code*`。

`response` 包可以继续作为 envelope DTO 的导入位置，但错误码、应用错误和分页契约必须从各自主包导入。

## Swagger Compatibility

将 user list response doc DTO 从 `response.Pagination` 改为 `pagination.Pagination` 后，Swagger definition key 可能从 `response.Pagination` 变为 `pagination.Pagination`。这不应影响实际 HTTP JSON 字段，但生成产物会有 definition 名变化。

实现后应运行 `make swagger-generate`。如果 Swagger 生成环境不可用，需要至少运行 Go 测试并记录未生成原因。验收重点是 JSON 字段兼容，而不是 Swagger definition key 保持旧名。

## Documentation Updates

`docs/ARCHITECTURE.md` 的 Common Organization 应更新为：

- `common/contract/errors`：稳定应用错误码和可渲染应用错误。
- `common/contract/pagination`：稳定 Cursor/Keyset 分页响应模型。
- `common/contract/response`：HTTP 响应信封 DTO 与默认消息。
- `common/http/response`：Gin HTTP 响应输出适配层。

`AGENTS.md` 中 Repository Shape 的 `common/` 分类说明已经包含 `contract`，如需要可补充 `errors` 与 `pagination` 是稳定契约包。

## Compatibility

保持兼容的内容：

- HTTP envelope 字段不变。
- 错误码数值不变。
- 错误码语义不变。
- HTTP status 映射不变。
- `errors.Is`/`errors.As` 对应用错误 wrapper 的行为不变。
- 分页字段、默认值、上限和 nil items 空数组行为不变。
- Gin writer 调用统一迁移到 `common/http/response`。

允许变化的内容：

- Go package 主定义位置变化。
- Swagger definition key 可能从 `response.Pagination` 变成 `pagination.Pagination`。
- 部分 Go import 从 `response` 转为 `errors` 或 `pagination`。

## Verification Strategy

- `common/contract/errors`：
  - 校验错误码数值。
  - 校验每个错误构造函数的 code、HTTP status、message。
  - 校验 `WrapInternal` 和 `Wrap` 的 `errors.Is`。
  - 校验 `FromError(nil)`、`FromError(appErr)` 和 `FromError(ordinaryErr)`。
- `common/contract/pagination`：
  - 校验分页大小默认值、负数处理、正常值保留、超过上限裁剪。
  - 校验 `NewPagination` 字段。
  - 校验 `NewPaginatedData(nil, meta)` 输出非 nil 空 slice。
- `common/contract/response`：
  - 保留 HTTP envelope JSON DTO 测试。
- `common/http/response`：
  - 覆盖 `OK`、`Fail`、`ValidationFailedWithErrors` 和错误状态映射。
- 模块测试：
  - 在 `common/` 执行 `go test ./...`。
  - 在 `user-service/` 执行 `go test ./...`。
- 文档和生成：
  - 运行 `make swagger-generate` 或记录未运行原因。
  - 检查 `docs/ARCHITECTURE.md` 与 `AGENTS.md` 是否仍禁止新增 OpenSpec/OPSX 工件。
