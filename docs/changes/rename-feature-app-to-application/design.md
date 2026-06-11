# Design

## Overview

本变更是一次受控的 feature 应用层目录和 Go package rename。应用层职责不变，只把当前短名 `app` 改为完整命名 `application`。

目标依赖方向：

```text
features/*/transport/http
  -> features/*/application
  -> features/*/domain

features/*/infra/postgres|redis
  -> features/*/application ports
  -> features/*/domain

features/*/module.go
  -> feature-local application, transport, infra
```

`application` 仍是 feature 内部业务编排层，继续拥有 service、command/query、ports 和 transport-neutral result。`transport/http`、`infra/postgres`、`infra/redis` 和 `module.go` 只更新 import path 与 package qualifier。

## Target Package Layout

```text
user-service/internal/features/user/
  api/
  application/
    commands.go
    ports.go
    result.go
    service.go
    service_test.go
  domain/
  infra/postgres/
  transport/http/
  module.go

user-service/internal/features/auth/
  api/
  application/
    commands.go
    components_test.go
    credentials.go
    ports.go
    result.go
    service.go
    service_test.go
    sessions.go
    tokens.go
  domain/
  infra/postgres/
  infra/redis/
  transport/http/
  module.go
```

No new subdirectories are introduced under `application`.

## Package Naming

Moved files should use:

```go
package application
```

Callers should update import paths:

```go
import userapplication "github.com/aegiscore/user-service/internal/features/user/application"
import authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
```

Aliases are recommended where both feature application packages may appear near each other or where the old package qualifier would otherwise leave stale `userapp` or `authapp` names. Inside each application package, tests that currently use `package app` should become `package application`.

## File Moves

Move user application files:

- `user-service/internal/features/user/app/commands.go`
- `user-service/internal/features/user/app/ports.go`
- `user-service/internal/features/user/app/result.go`
- `user-service/internal/features/user/app/service.go`
- `user-service/internal/features/user/app/service_test.go`

Move auth application files:

- `user-service/internal/features/auth/app/commands.go`
- `user-service/internal/features/auth/app/components_test.go`
- `user-service/internal/features/auth/app/credentials.go`
- `user-service/internal/features/auth/app/ports.go`
- `user-service/internal/features/auth/app/result.go`
- `user-service/internal/features/auth/app/service.go`
- `user-service/internal/features/auth/app/service_test.go`
- `user-service/internal/features/auth/app/sessions.go`
- `user-service/internal/features/auth/app/tokens.go`

Use `git mv` or an equivalent tracked move during implementation so review clearly shows this is a rename-first change.

## Caller Migration

Known caller groups:

- `user-service/internal/features/user/module.go`
- `user-service/internal/features/auth/module.go`
- `user-service/internal/features/user/transport/http/*.go`
- `user-service/internal/features/auth/transport/http/*.go`
- `user-service/internal/features/user/infra/postgres/*.go`
- `user-service/internal/features/auth/infra/postgres/*.go`
- `user-service/internal/features/auth/infra/redis/*.go`
- `user-service/internal/providers/routes_test.go`

Migration is mechanical:

1. Replace `/features/<feature>/app` import paths with `/features/<feature>/application`.
2. Replace package qualifiers from `userapp` and `authapp` to `userapplication` and `authapplication`.
3. Update Fx `fx.As(new(...))` annotations to refer to the renamed application package.
4. Update tests, mocks and fixtures that refer to application layer types.
5. Run `gofmt` on all changed Go files.

Controller behavior should remain unchanged: controllers still map HTTP DTOs to application command/query types before calling services. Infra adapter behavior should remain unchanged: adapters still implement application-owned ports and map storage models to domain/application results.

## Documentation Updates

Update current long-lived docs:

- `AGENTS.md`
  - Repository Shape user/auth feature layouts should list `application/`.
  - Key Entry Points should point user/auth service entries to `application/service.go`.
  - Repository Rules and dependency table should use `application` instead of `app`.
  - Ports rule should point to `internal/features/<feature>/application/ports.go`.
  - Ent predicate rule should refer to `application/service.go`.
- `docs/ARCHITECTURE.md`
  - HTTP Request Flow business call location should become `features/*/application/`.
  - Feature-First Organization table should rename `app/` to `application/`.
  - Dependency rules should use `application`.
  - Ports and controller mapping guidance should use application layer naming.
  - Integration relationship text should say feature application service and application ports.
- `docs/DEVELOPMENT.md`
  - Coding conventions and adding-feature guidance should use `application`.

Historical `docs/changes/*` records can remain as historical context unless they are being edited as part of the current change. Acceptance scans should focus on current code and long-lived docs, not on older change records that document prior repository states.

## Behavior Preservation

The implementation must not change:

- Public HTTP paths or HTTP methods.
- Request/response DTO fields.
- Response envelope shape.
- App/application service method signatures except for package qualification in callers.
- Command/query/result fields.
- Port interface method sets.
- Domain errors and value objects.
- Ent predicates or storage query semantics.
- Redis key construction, session serialization or token validation semantics.
- Fx graph composition beyond package path references.

## Verification Strategy

Run structural checks:

```bash
test ! -d user-service/internal/features/user/app
test ! -d user-service/internal/features/auth/app
test -d user-service/internal/features/user/application
test -d user-service/internal/features/auth/application
rg -n '/app"|features/(user|auth)/app(/|$)|^package app$|\buserapp\b|\bauthapp\b' user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md
```

Run Go verification:

```bash
cd user-service
go test ./...
```

If import edits touch shared docs only, `common/` tests are not required for this change because `common` code is not in scope.

## Risks And Mitigations

### Missed imports

Risk: a package still imports `/app`, causing compile failures or stale naming.

Mitigation: run `go test ./...` and targeted `rg` scans over `user-service` and current docs.

### Hidden behavior drift

Risk: while touching many files, implementation accidentally changes service logic, adapter mappings or test expectations.

Mitigation: keep edits mechanical, review diff around non-import lines, and avoid unrelated refactors.

### Historical docs confusion

Risk: older change records may still mention `app` and confuse broad searches.

Mitigation: update long-lived docs used as current rules. Treat prior `docs/changes/*` entries as historical unless a stale reference is presented as current guidance.
