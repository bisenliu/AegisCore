# Design

## Overview

本变更把 user feature application 层从单一 service 文件拆成按用例类别组织的内部子包：

```text
transport/http
  -> application/command
  -> application/query
  -> application
  -> domain

infrastructure/postgres
  -> application ports
  -> domain
```

`application/command` 负责写用例，`application/query` 负责读用例，`application/validators` 负责 application 层输入前置校验或规范化辅助。`application` 根包继续拥有稳定的跨用例契约：ports 和跨用例 input structs。Result DTO 跟随具体 use case 放在 command/query 文件中。

HTTP controller 仍然只处理 HTTP 绑定、DTO 清洗、路径/查询参数解析、HTTP 错误映射和响应输出。Application command/query 只接收 transport-neutral command/query 类型并返回对应 use-case-local result。

## Target Package Layout

```text
user-service/internal/features/user/application/
  command/
    create_user.go
    create_user_test.go
  query/
    get_user.go
    list_users.go
    query_service.go
    query_service_test.go
  validators/
    user_validator.go
    user_validator_test.go
  ports.go
```

Possible root package files after migration:

- `ports.go` keeps `UserProfileStore`, `CreateUserInput` and `ListUsersInput` unless implementation discovers a cleaner root-level split.
- Result types live next to their owning use case, for example `CreateUserResult` in `command/create_user.go`, `GetUserResult` in `query/get_user.go`, and `ListUsersResult` in `query/list_users.go`.
- A small root-level facade or interface file may remain if needed to preserve controller construction ergonomics, but it must not reintroduce the full monolithic service behavior or a catch-all result registry.

## Command Layer

`application/command` owns write-side use cases.

Initial content:

- `CreateUserCommand`
- `CreateUserResult`
- `CreateUserService`
- `NewCreateUserService`
- `CreateUser(ctx, cmd)` method

Responsibilities:

- Resolve default user status when command status is omitted.
- Invoke validators for create-user application input.
- Hash the password through `common/security/password`.
- Generate a V7 UUID.
- Call the application-owned `UserProfileStore.Create` port with `application.CreateUserInput`.
- Map `domain.ErrUserAlreadyExists` without translating it to HTTP errors.
- Log command execution and failures with existing logger patterns.

The command package may import:

- `context`
- standard library helpers needed by the use case
- `common/runtime/logger`
- `common/security/password`
- `user/application`
- `user/application/validators`
- `user/domain`
- `github.com/google/uuid`
- `go.uber.org/zap`

The command package must not import Gin, HTTP binder, response envelope, Ent, Redis or SQL.

## Query Layer

`application/query` owns read-side use cases.

Initial content:

- `GetUserByIDQuery` or an equivalent transport-neutral input type if method signatures should avoid raw UUID arguments.
- `GetUserResult`
- `ListUsersQuery`
- `ListUsersResult`
- `UserQueryService`
- `NewUserQueryService`
- `GetUserByID(ctx, query)` or `GetUserByID(ctx, userID)` method
- `ListUsers(ctx, query)` method

Responsibilities:

- Log read operations with existing field names.
- Invoke validators for query inputs where application-level checks are needed.
- Call `UserProfileStore.GetByUserID` and `UserProfileStore.ListUsers`.
- Preserve list pagination behavior:
  - `AfterUserID` is the cursor.
  - `Limit` is passed through from normalized HTTP query input.
  - `NextCursor` is the last returned user ID only when `hasNext` is true and at least one item exists.
  - `PageSize` mirrors the normalized request page size.
- Preserve `domain.ErrUserNotFound` handling for get-by-ID.

The query package may import:

- `context`
- standard library helpers needed by the use case
- `common/runtime/logger`
- `user/application`
- `user/application/validators`
- `user/domain`
- `github.com/google/uuid`
- `go.uber.org/zap`

The query package must not import Gin, HTTP binder, response envelope, Ent, Redis or SQL.

## Validators Layer

`application/validators` owns transport-neutral application input checks that are not HTTP binding rules.

Initial validators should be deliberately small. HTTP request shape validation, field labels, JSON binding and query parsing stay in `transport/http`.

Good candidates:

- Create command status default validation if command layer needs an explicit guard.
- List query limit/page-size consistency checks if future non-HTTP callers can construct queries directly.
- Defensive user ID or cursor nil checks where application behavior would otherwise be ambiguous.

Non-goals:

- Do not duplicate Gin validator tags from HTTP DTOs.
- Do not return HTTP errors or response envelopes.
- Do not import HTTP request/response DTOs.
- Do not call stores, hash passwords, generate tokens or orchestrate use cases.

Validators should return domain or application-level errors, leaving HTTP mapping in `transport/http`.

## Root Application Contracts

`application/ports.go` should remain the consumer-owned port boundary for user feature storage:

```go
type UserProfileStore interface {
    Create(ctx context.Context, input CreateUserInput) (*domain.User, error)
    GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
    ListUsers(ctx context.Context, input ListUsersInput) ([]domain.User, bool, error)
}
```

This keeps the PostgreSQL adapter dependency direction unchanged:

```text
infrastructure/postgres -> application -> domain
```

If implementation splits read and write ports, those ports should still live under the application layer and be owned by the consuming command/query packages. The PostgreSQL adapter may implement both interfaces, but it must not define interfaces for its own convenience.

Result DTOs should stay with their owning use case instead of accumulating in a root `result.go`. This keeps feature-first CQRS-lite files movable as a unit: query input, result and handler live together.

## Controller Wiring

`transport/http.UserController` should depend on explicit command/query services instead of a monolithic `UserService`.

Expected shape:

```go
type UserController struct {
    commands command.UserCommands
    queries  query.UserQueries
    validator *commonvalidation.Validator
}
```

Equivalent names are acceptable if they match existing style. The controller methods should continue to:

- Bind HTTP DTOs with `common/http/binding`.
- Normalize `CreateUserRequest` and `ListUsersRequest`.
- Parse user IDs and cursors in `transport/http`.
- Construct command/query inputs from application packages.
- Call command/query services.
- Map domain/application errors through `toUserHTTPError`.
- Render responses with `common/http/response`.

The controller must not pass HTTP request/response DTOs into application services.

## Fx Wiring

`user-service/internal/features/user/fx.go` should provide the new command and query services explicitly.

Expected composition:

```go
var Module = fx.Module("feature-user",
    fx.Provide(
        fx.Annotate(
            userpostgres.NewUserStore,
            fx.As(new(application.UserProfileStore)),
        ),
        command.NewCreateUserService,
        query.NewUserQueryService,
        userhttp.NewUserController,
    ),
)
```

If interfaces are used for controller dependencies, annotate the command/query constructors with `fx.As` as needed. Keep service-level providers in `internal/providers` out of this feature-local change.

## Testing Strategy

Move and refocus current application tests:

- Create-user tests move to `application/command/create_user_test.go`.
- Get-user and list-users tests move to `application/query/query_service_test.go` or focused per-use-case test files.
- Validator-specific tests live in `application/validators`.
- Existing controller tests should continue to assert:
  - create request mapping
  - list query normalization and cursor parsing
  - get-by-ID path parsing
  - domain error to HTTP error mapping
  - response envelope shape

Controller test stubs should reflect the split dependencies. Prefer small command/query stubs over a single stub that implements every use case.

## Documentation Updates

Update current long-lived docs:

- `AGENTS.md`
  - Repository Shape should mention user application subdirectories.
  - Repository Rules should allow feature application layer to split into command/query/validators while preserving dependency rules.
  - Key Entry Points should point to the new command/query service files.
- `docs/ARCHITECTURE.md`
  - Feature-first organization should describe optional `application/command`, `application/query` and `application/validators` subdivision.
  - HTTP request flow should note controller mapping to command/query use cases.
- `docs/DEVELOPMENT.md`
  - Development guidance should describe where new user read/write use cases go.
- `docs/TESTING.md`
  - If it names the user application service tests, update paths to command/query tests.

Historical `docs/changes/*` records can remain as historical context unless they are part of this change.

## Behavior Preservation

The implementation must preserve:

- Public HTTP paths and methods.
- Request/response DTO fields.
- Response envelope shape.
- Status codes and error codes.
- Username normalization currently done in HTTP request normalization.
- Default user status behavior.
- Password hashing behavior.
- UUID V7 generation behavior.
- User-not-found and user-already-exists error semantics.
- Cursor pagination behavior.
- PostgreSQL predicates, sorting, soft-delete filtering and limit-plus-one fetch behavior.
- Ent schema, generated code and migrations.

## Verification Strategy

Run focused checks:

```bash
cd user-service
go test ./internal/features/user/...
```

Run broader service tests when imports or Fx wiring change:

```bash
cd user-service
go test ./...
```

Run structural checks:

```bash
test -d user-service/internal/features/user/application/command
test -d user-service/internal/features/user/application/query
test -d user-service/internal/features/user/application/validators
rg -n 'type service struct|func \\(s \\*service\\) CreateUser|func \\(s \\*service\\) GetUserByID|func \\(s \\*service\\) ListUsers' user-service/internal/features/user/application
```

The final `rg` should not show a monolithic root application service still implementing all three use cases.

## Risks And Mitigations

### Import cycles

Risk: `application/command` and `application/query` may import root `application`, while root `application` imports command/query for facade interfaces, creating a cycle.

Mitigation: keep root contracts limited to ports and cross-use-case input structs that do not import command/query. If a facade interface is needed, define it in the consumer package or in a file that does not require importing subpackages.

### Over-splitting small logic

Risk: validators become a dumping ground for tiny transport rules already handled by HTTP DTO validation.

Mitigation: only move transport-neutral application checks into validators. Keep HTTP binding, field labels, query parsing and DTO normalization in `transport/http`.

### Fx constructor mismatch

Risk: controller constructor signatures change but Fx providers are not annotated correctly.

Mitigation: update `fx.go` and run `go test ./...` so Fx graph compile-time wiring and provider tests catch mismatches.

### Behavior drift during movement

Risk: splitting code changes pagination, default status, password hashing or domain error handling.

Mitigation: move existing tests first, keep assertions intact, then adapt stubs and constructors. Add only focused tests for new validators.
