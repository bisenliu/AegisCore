# Design

## Overview

本变更是一次受控的 feature Fx 入口文件 rename。Feature package、导出变量、Fx module 名称和 provider wiring 全部保持不变，只把入口文件名从通用的 `module.go` 改为明确表达 Fx 组装职责的 `fx.go`。

目标依赖方向不变：

```text
bootstrap.AppModule
  -> features/auth.Module
  -> features/auth fx wiring

bootstrap.AppModule
  -> features/user.Module
  -> features/user fx wiring
```

Feature-local Fx 入口仍只负责组装本 feature 的 application service、transport controller 和 infrastructure adapter，不承载业务逻辑。

## Target Package Layout

```text
user-service/internal/features/user/
  api/
  application/
  domain/
  infrastructure/postgres/
  transport/http/
  fx.go

user-service/internal/features/auth/
  api/
  application/
  domain/
  infrastructure/postgres/
  infrastructure/redis/
  transport/http/
  fx.go
```

No new directories are introduced.

## File Moves

Move user feature Fx entry:

- `user-service/internal/features/user/module.go`
- `user-service/internal/features/user/fx.go`

Move auth feature Fx entry:

- `user-service/internal/features/auth/module.go`
- `user-service/internal/features/auth/fx.go`

Use `git mv` or an equivalent tracked move during implementation so review clearly shows this is a rename-first change.

## Exported API

Keep the exported variable name:

```go
var Module = fx.Module(...)
```

The variable should remain named `Module` because package-qualified call sites already read clearly:

```go
authfeature.Module
userfeature.Module
```

Renaming it to `FxModule`, `FeatureModule` or similar would create avoidable API churn without improving the import sites. The file name is the thing being standardized, not the package API.

## User Feature Wiring

`user-service/internal/features/user/fx.go` should keep the current package and provider graph:

- `package user`
- import user application package
- import user PostgreSQL infrastructure package
- import user HTTP transport package
- import `go.uber.org/fx`
- `fx.Module("feature-user", ...)`
- `userpostgres.NewUserStore` annotated as `userapplication.UserProfileStore`
- `userapplication.NewUserService`
- `userhttp.NewUserController`

No provider should be added, removed or reordered.

## Auth Feature Wiring

`user-service/internal/features/auth/fx.go` should keep the current package and provider graph:

- `package auth`
- import auth application package
- import auth domain package
- import auth PostgreSQL infrastructure package
- import auth Redis infrastructure package
- import auth HTTP transport package
- import `go.uber.org/fx`
- `fx.Module("feature-auth", ...)`
- `authpostgres.NewCredentialStore` annotated as `authapplication.UserCredentialStore`
- `authpostgres.NewCredentialStore` annotated as `authapplication.UserTokenVersionStore`
- `authdomain.NewRedisKeyBuilder`
- `authredis.NewSessionStore` annotated as `authapplication.AuthSessionStore`
- `authapplication.NewTokenVersionValidator`
- `authapplication.NewAuthService`
- `authhttp.NewAuthController`

No provider should be added, removed or reordered.

## Bootstrap Interaction

`bootstrap.AppModule` should not require source changes if it imports the feature packages rather than the feature files. The exported package-level value remains `Module`, so existing call sites should continue to compile:

```go
authfeature.Module
userfeature.Module
```

If any comments or tests refer to feature `module.go` by file path, update those references to `fx.go`. Do not introduce forwarding files named `module.go`.

## Documentation Updates

Update current long-lived docs:

- `AGENTS.md`
  - Repository Shape user/auth feature layouts should list `fx.go`.
  - Key Entry Points should point user/auth feature module entries to `fx.go`.
  - Repository Rules should say each feature exposes Fx wiring from `features/<feature>/fx.go`.
  - Dependency table should rename the feature Fx row from `module.go` to `fx.go`.
- `docs/ARCHITECTURE.md`
  - Feature-First Organization table should list `fx.go` as the feature-local Fx module entry.
  - Dependency Rules should use `fx.go` for the Fx-only feature wiring layer.
  - Runtime flow can keep saying feature modules, but file-specific examples should use `fx.go`.
- `docs/DEVELOPMENT.md`
  - Adding-feature or coding-convention guidance should mention `fx.go` when describing the feature directory shape.

Historical `docs/changes/*` records can remain as historical context unless they are being edited as part of the current change. Acceptance scans should focus on current code and long-lived docs, not older change records that documented prior repository states.

## Behavior Preservation

The implementation must not change:

- Public HTTP paths or HTTP methods.
- Request/response DTO fields.
- Response envelope shape.
- Application service method signatures.
- Command/query/result fields.
- Port interface method sets.
- Domain errors and value objects.
- Ent predicates or storage query semantics.
- Redis key construction, session serialization or token validation semantics.
- Fx provider order or dependencies.
- Fx module names.
- Feature package names.

## Verification Strategy

Run structural checks:

```bash
test ! -f user-service/internal/features/user/module.go
test ! -f user-service/internal/features/auth/module.go
test -f user-service/internal/features/user/fx.go
test -f user-service/internal/features/auth/fx.go
rg -n 'features/.+/module[.]go|module[.]go' AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md user-service/internal/features
```

The `rg` command should return no current feature Fx entry references. If it finds historical change records, those should be excluded from this specific acceptance check rather than rewritten for history.

Run Go verification:

```bash
cd user-service
go test ./...
```

If implementation only renames feature Fx files and long-lived docs, `common/` tests are not required because `common` code is not in scope.

## Risks And Mitigations

### Stale file-path references

Risk: docs or tests still refer to `features/<feature>/module.go`, making the final structure ambiguous.

Mitigation: run targeted `rg` scans over `AGENTS.md`, long-lived docs and `user-service/internal/features`.

### Accidental Fx graph drift

Risk: while moving files, provider order, annotations or dependencies change.

Mitigation: use rename-first edits, inspect the diff of `fx.go`, and run `go test ./...` in `user-service`.

### Unnecessary API churn

Risk: renaming the exported `Module` variable forces unrelated bootstrap and test changes.

Mitigation: keep `Module` as the exported package API and only standardize the file name.
