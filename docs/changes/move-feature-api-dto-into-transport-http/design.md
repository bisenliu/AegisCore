# Design

## Overview

本变更是一次受控的 HTTP DTO 归属迁移。现有 `features/user/api` 和 `features/auth/api` 包中的类型只被 HTTP transport 使用，因此将它们移动到对应的 `features/<feature>/transport/http` 包。

目标依赖方向：

```text
features/*/transport/http
  -> features/*/application
  -> features/*/domain

features/*/infrastructure/postgres|redis
  -> features/*/application ports
  -> features/*/domain
```

迁移后 `transport/http` 同时拥有 Gin controller、route registration、request binding DTO、response DTO、Swagger doc model、validation 和 mapper。Application 层继续拥有 command/query/result 和 ports，不依赖 HTTP DTO。

## Target Package Layout

```text
user-service/internal/features/user/transport/http/
  controller.go
  controller_test.go
  mapper.go
  request.go
  response.go
  routes.go
  validation.go
  validation_test.go

user-service/internal/features/auth/transport/http/
  controller.go
  controller_test.go
  mapper.go
  request.go
  response.go
  routes.go
  validation.go
  validation_test.go
```

No replacement `api/`, `dto/`, `models/` or shared HTTP DTO directory should be introduced.

## Package Naming

Moved files should use the existing transport package names:

```go
package userhttp
package authhttp
```

Because the DTOs move into the same package as controller, mapper and validation code, most call sites should drop the old package qualifier instead of introducing a new alias.

Examples:

```go
req := CreateUserRequest{}
func NormalizeCreateUser(req *CreateUserRequest) error
func toTokenResponse(result *authapplication.TokenResult) TokenResponse
```

Tests in the same package should also use `CreateUserRequest`, `LoginRequest`, and related types directly.

## File Moves

Move user DTO files:

- `user-service/internal/features/user/api/request.go` -> `user-service/internal/features/user/transport/http/request.go`
- `user-service/internal/features/user/api/response.go` -> merge into or move to `user-service/internal/features/user/transport/http/response.go`
- `user-service/internal/features/user/api/doc.go` -> merge into `user-service/internal/features/user/transport/http/response.go`

Move auth DTO files:

- `user-service/internal/features/auth/api/request.go` -> `user-service/internal/features/auth/transport/http/request.go`
- `user-service/internal/features/auth/api/response.go` -> `user-service/internal/features/auth/transport/http/response.go`

Use `git mv` or an equivalent tracked move during implementation so review clearly shows this is a move-first change. When moving user `doc.go`, prefer folding `UserResponseDoc` and `UserListResponseDoc` into `response.go` rather than leaving a separate doc-only file.

## Caller Migration

Known caller groups:

- `user-service/internal/features/user/transport/http/controller.go`
- `user-service/internal/features/user/transport/http/mapper.go`
- `user-service/internal/features/user/transport/http/validation.go`
- `user-service/internal/features/user/transport/http/validation_test.go`
- `user-service/internal/features/auth/transport/http/controller.go`
- `user-service/internal/features/auth/transport/http/mapper.go`
- `user-service/internal/features/auth/transport/http/validation.go`
- `user-service/internal/features/auth/transport/http/validation_test.go`

Migration is mechanical:

1. Remove imports of `features/<feature>/api` from HTTP transport files.
2. Replace `userapi.TypeName` with `TypeName` inside `package userhttp`.
3. Replace `authapi.TypeName` with `TypeName` inside `package authhttp`.
4. Update Swagger annotations to reference the local package name that swag can resolve from controller comments.
5. Run `gofmt` on all changed Go files.

Swagger annotations should use the moved DTO type names. For example:

```go
// @Param request body userhttp.CreateUserRequest true "创建用户请求"
// @Success 201 {object} response.Envelope{data=userhttp.UserResponseDoc} "创建成功"
// @Param request body authhttp.LoginRequest true "登录请求"
// @Success 200 {object} response.Envelope{data=authhttp.TokenResponse} "登录成功"
```

If Swagger generation accepts unqualified local type names in this package, that is also acceptable, but the generated docs must preserve the same JSON schema fields.

## User DTO Details

`request.go` should contain:

- `GetUserRequest`
- `ListUsersRequest`
- `CreateUserRequest`
- `(*CreateUserRequest).SetDefaults`

`response.go` should contain:

- `UserResponse`
- `UserResponseDoc`
- `UserListResponseDoc`

Field definitions and tags must be copied unchanged. `ListUsersRequest.Status` and `CreateUserRequest.Status` may continue to depend on `userdomain.UserStatus` because HTTP binding currently validates the domain enum at the transport boundary.

`UserResponse` may continue to use `userdomain.UserStatus` for the runtime response value, and `UserResponseDoc` may continue to use `int64` for Swagger clarity.

## Auth DTO Details

`request.go` should contain:

- `LoginRequest`
- `RefreshTokenRequest`
- `ChangePasswordRequest`

`response.go` should contain:

- `TokenResponse`
- `LogoutResponse`
- `ChangePasswordResponse`

Field definitions and tags must be copied unchanged. `ChangePasswordRequest.Token` must remain `json:"-"` and continue to be populated from the `Authorization` header in the controller before JSON binding.

## Documentation Updates

Update current long-lived docs:

- `AGENTS.md`
  - Repository Shape should describe user/auth feature layers without `api/`.
  - Repository Rules should state HTTP request/response DTO live in `transport/http/request.go` and `transport/http/response.go`.
  - Dependency table should allow `transport/http` to depend on application, Gin, response envelope and feature-local DTO/validation, not a sibling `api` package.
  - Key Entry Points should mention user/auth `transport/http/request.go` and `response.go` if DTO entry points are listed.
- `docs/ARCHITECTURE.md`
  - Feature-First Organization table should remove `api/` and make HTTP DTO ownership part of `transport/http`.
  - HTTP Request Flow should keep request parsing at `features/*/transport/http/controller.go`.
  - Dependency Rules should no longer list `api` as a transport dependency.
- `docs/DEVELOPMENT.md`
  - Coding conventions and adding-feature guidance should use `application/domain/transport/http/infrastructure/*/fx.go` and place HTTP DTO in transport.

Historical `docs/changes/*` records can remain as historical context unless they are being edited as part of the current change. Acceptance scans should focus on current code and long-lived docs, not old completed change records.

## Behavior Preservation

The implementation must not change:

- Public HTTP paths or HTTP methods.
- Request body, query, URI or response JSON field names.
- Validation tags, label tags or example tags.
- Response envelope shape.
- Controller-to-application command/query mapping.
- Application service method signatures.
- Command/query/result fields.
- Domain errors and value objects.
- Ent predicates, storage behavior, Redis behavior, JWT/session/token version semantics.
- Fx graph composition.
- Config keys, runtime resource names, Ent schema, generated code or migration files.

## Verification Strategy

Run structural checks:

```bash
test ! -d user-service/internal/features/user/api
test ! -d user-service/internal/features/auth/api
test -f user-service/internal/features/user/transport/http/request.go
test -f user-service/internal/features/user/transport/http/response.go
test -f user-service/internal/features/auth/transport/http/request.go
test -f user-service/internal/features/auth/transport/http/response.go
rg -n 'features/(user|auth)/api|\buserapi\b|\bauthapi\b' user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md
```

Run Go and Swagger verification:

```bash
cd user-service
go test ./internal/features/user/transport/http ./internal/features/auth/transport/http
cd ..
make swagger-generate
```

If the implementation touches only user-service HTTP transport and long-lived docs, `common/` tests are not required because common code is not in scope.

## Risks And Mitigations

### Swagger type resolution

Risk: after moving DTOs into `package userhttp` and `package authhttp`, Swagger annotations may still reference old `userapi` or `authapi` types or may not resolve newly qualified names.

Mitigation: update annotations together with code imports, run `make swagger-generate`, and inspect the generated schema diff for unchanged JSON fields.

### Boundary drift

Risk: implementation moves DTOs but then allows application service APIs to accept transport DTOs because the package is now nearby.

Mitigation: keep application command/query/result types unchanged, run import scans to ensure application/domain do not import `transport/http`, and review controller mapping code.

### Hidden API drift

Risk: copying DTOs into new files accidentally changes JSON tags, validation tags or field types.

Mitigation: use move-first edits, review DTO diffs carefully, and rely on controller tests plus Swagger regeneration to catch schema drift.

### Stale package directories

Risk: `api/` remains with an empty `doc.go` or leftover package comment.

Mitigation: remove the empty directories and run targeted `rg` scans for `features/(user|auth)/api`, `userapi` and `authapi`.
