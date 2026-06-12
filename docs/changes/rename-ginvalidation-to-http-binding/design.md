# Design

## Overview

本变更是一次受控 Go package rename。当前 `common/http/ginvalidation` 已经位于 HTTP/Gin 边界层，内部只做：

- 从 Gin context 读取 URI、query、form 或 JSON body。
- 调用 `common/validation` 进行值绑定、错误归一化和结构体校验。
- 在 `BindOrAbort` 失败时通过 `common/http/response` 写出统一错误 envelope。
- 记录 invalid request 日志并中止 Gin context。

新包 `common/http/binding` 继续承担同一职责。实现内容保持等价，只调整目录、package 名和调用方 import。

## Package Layout

目标结构：

```text
common/http/
  binding/
    binder.go
    validator.go
    validation_test.go
  middleware/
  response/
```

`binding` 包仍属于 `common/http`，因此可以依赖 Gin 和 `common/http/response`。它不进入 `common/validation`，因为 `common/validation` 必须保持不依赖 Gin。

## API Surface

迁移后保留同名导出符号：

```go
type Binder func(*gin.Context, any) error

func URIBinder(c *gin.Context, dst any) error
func QueryBinder(c *gin.Context, dst any) error
func JSONBinder(c *gin.Context, dst any) error
func StrictJSONBinder(c *gin.Context, dst any) error
func JSONBinderWithOptions(disallowUnknownFields bool) Binder
func FormBinder(c *gin.Context, dst any) error

func Bind(validator *validation.Validator, c *gin.Context, dst any, binder Binder) error
func BindOrAbort(validator *validation.Validator, c *gin.Context, dst any, binder Binder) bool
```

Only the import path and package qualifier change:

```go
import "github.com/aegiscore/common/http/binding"

if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
    return
}
```

No compatibility alias package is planned. The repository owns all known callers, and keeping a `ginvalidation` shim would preserve the confusing name this change removes.

## Caller Migration

Known current callers:

- `user-service/internal/features/user/transport/http/controller.go`
- `user-service/internal/features/auth/transport/http/controller.go`

Migration is mechanical:

1. Change import path from `github.com/aegiscore/common/http/ginvalidation` to `github.com/aegiscore/common/http/binding`.
2. Change qualifier from `ginvalidation.` to `binding.`.
3. Run `gofmt` on modified Go files.

Feature-local `transport/http/validation.go` files stay unchanged. They parse and normalize DTO inputs at the feature boundary and should not absorb package-level binding behavior.

## Behavior Preservation

The moved tests should continue to cover:

- JSON body binding.
- Empty JSON body handling.
- JSON type mismatch normalization.
- Trailing JSON document rejection.
- Unknown field compatibility and strict rejection.
- URI, query and form binding.
- Text unmarshaler and `time.Duration` value binding.
- Embedded pointer struct binding.
- `BindOrAbort` failure response and logging behavior.
- Bad request handling for type mismatch without field-level validation errors.

The implementation must not change:

- `validation.ErrEmptyRequestBody`
- `validation.ErrTrailingJSONBody`
- bad request vs validation failed classification
- `contract/errors` codes
- response envelope shape
- logger message `invalid request`
- log fields `path`, `error`, and conditional `errors`
- Gin context abort behavior

## Documentation Updates

Long-term structure docs should describe the new path:

- `AGENTS.md` repository shape and rules, if mentioning this adapter directly.
- `docs/ARCHITECTURE.md` common organization and HTTP request flow.
- `docs/DEVELOPMENT.md`, if it references the old package in development guidance.
- `docs/TESTING.md`, if it references package-specific validation or binding tests.

Historical `docs/changes/*` records may retain old paths as historical context. They are not current rules unless referenced by `AGENTS.md` or `docs/ARCHITECTURE.md`.

## Verification Strategy

Run targeted checks first:

```bash
rg -n "ginvalidation" common user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md
cd common && go test ./http/binding ./validation
cd user-service && go test ./internal/features/user/transport/http ./internal/features/auth/transport/http ./internal/bootstrap
```

If time allows, run the broader module tests:

```bash
make test-common
make test-user-service
```

The `rg` command may still find historical references under `docs/changes/*` if scanned broadly; those are acceptable only as historical records, not active implementation or long-term rules.

## Risk

Risk is low because this is a package rename with existing test coverage around the critical behavior. The main failure modes are missed imports, stale docs, or accidentally moving logic into `common/validation` or feature-local validation files. Targeted `rg`, `go test`, and `gofmt` should catch the practical issues.
