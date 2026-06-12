# Design

## Overview

本变更是一次受控的 feature infrastructure 层目录 rename。基础设施 adapter 职责不变，只把当前短名 `infra` 改为完整命名 `infrastructure`。

目标依赖方向：

```text
features/*/transport/http
  -> features/*/application
  -> features/*/domain

features/*/infrastructure/postgres|redis
  -> features/*/application ports
  -> features/*/domain

features/*/module.go
  -> feature-local application, transport, infrastructure
```

`infrastructure` 仍是 feature 内服务拥有资源 adapter 层，继续承载 Ent/PostgreSQL adapter、Redis adapter、predicate 构造、存储模型转换和存储错误转换。`transport/http`、`application`、`domain` 和 `module.go` 只更新 import path 与分层称谓。

## Target Package Layout

```text
user-service/internal/features/user/
  api/
  application/
  domain/
  infrastructure/
    postgres/
      predicates.go
      user_store.go
      user_store_test.go
  transport/http/
  module.go

user-service/internal/features/auth/
  api/
  application/
  domain/
  infrastructure/
    postgres/
      credential_store.go
      credential_store_test.go
    redis/
      session_store.go
      session_store_test.go
  transport/http/
  module.go
```

No new subdirectories are introduced under `infrastructure`.

## Package Naming

Moved adapter files should keep their current Go package names:

```go
package postgres
package redis
```

Callers should update import paths:

```go
import userpostgres "github.com/aegiscore/user-service/internal/features/user/infrastructure/postgres"
import authpostgres "github.com/aegiscore/user-service/internal/features/auth/infrastructure/postgres"
import authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
```

Aliases are recommended in feature modules because both feature-specific infrastructure packages can appear near Fx wiring. Avoid aliases that preserve the old layer name, such as `userinfra` or `authinfra`.

## File Moves

Move user infrastructure files:

- `user-service/internal/features/user/infra/postgres/predicates.go`
- `user-service/internal/features/user/infra/postgres/user_store.go`
- `user-service/internal/features/user/infra/postgres/user_store_test.go`

Move auth PostgreSQL infrastructure files:

- `user-service/internal/features/auth/infra/postgres/credential_store.go`
- `user-service/internal/features/auth/infra/postgres/credential_store_test.go`

Move auth Redis infrastructure files:

- `user-service/internal/features/auth/infra/redis/session_store.go`
- `user-service/internal/features/auth/infra/redis/session_store_test.go`

Use `git mv` or an equivalent tracked move during implementation so review clearly shows this is a rename-first change.

## Caller Migration

Known caller groups:

- `user-service/internal/features/user/module.go`
- `user-service/internal/features/auth/module.go`
- Any tests or helpers that import feature infrastructure adapters directly.
- Current long-lived docs that mention `infra/postgres`, `infra/redis`, or `infra/*`.

Migration is mechanical:

1. Replace `/features/<feature>/infra/<resource>` import paths with `/features/<feature>/infrastructure/<resource>`.
2. Keep package qualifiers focused on the resource package, for example `userpostgres`, `authpostgres`, and `authredis`.
3. Update Fx provider references to the moved adapter constructors.
4. Update tests that import adapter packages.
5. Run `gofmt` on all changed Go files.

Application services must continue to consume ports from `features/<feature>/application/ports.go`. Infrastructure adapters must continue to implement those ports. Do not move ports, command/query types, result types, domain objects, or repository-like interfaces into `infrastructure`.

## Documentation Updates

Update current long-lived docs:

- `AGENTS.md`
  - Repository Shape user/auth feature layouts should list `infrastructure/postgres` and `infrastructure/redis`.
  - Key Entry Points should point user/auth adapter entries to `infrastructure/...`.
  - Repository Rules should use `infrastructure/*` and call the layer infrastructure adapter.
  - Dependency table should use `infrastructure/postgres` and `infrastructure/redis`.
  - Ports rule should still point to `application/ports.go` and clarify interfaces do not move into infrastructure.
  - Ent predicate rule should refer to `internal/features/user/infrastructure/postgres/predicates.go`.
- `docs/ARCHITECTURE.md`
  - HTTP Request Flow data access location should become `features/*/infrastructure/postgres/` and `features/*/infrastructure/redis/`.
  - Feature-First Organization table should rename `infra/postgres` and `infra/redis` to `infrastructure/postgres` and `infrastructure/redis`.
  - Dependency rules should use infrastructure layer naming.
  - Ports, adapter and integration relationship text should distinguish service-owned infrastructure adapters from external `internal/integration` adapters.
- `docs/DEVELOPMENT.md`
  - Coding conventions and adding-feature guidance should use `infrastructure`.
  - Keep references to adapter behavior intact; only the layer name changes.

Historical `docs/changes/*` records can remain as historical context unless they are being edited as part of the current change. Acceptance scans should focus on current code and long-lived docs, not on older change records that document prior repository states.

## Behavior Preservation

The implementation must not change:

- Public HTTP paths or HTTP methods.
- Request/response DTO fields.
- Response envelope shape.
- Application service method signatures.
- Command/query/result fields.
- Port interface method sets or ownership.
- Domain errors and value objects.
- Ent predicates, query filters, mutation fields or storage error mapping.
- Redis key construction, session serialization, token version lookup or expiration semantics.
- Fx graph composition beyond package path references.
- Config keys, runtime resource names, Ent schema, generated code or migration files.

## Verification Strategy

Run structural checks:

```bash
test ! -d user-service/internal/features/user/infra
test ! -d user-service/internal/features/auth/infra
test -d user-service/internal/features/user/infrastructure/postgres
test -d user-service/internal/features/auth/infrastructure/postgres
test -d user-service/internal/features/auth/infrastructure/redis
rg -n 'features/.*/infra(/|$)|/infra"|/infra/|infra/(postgres|redis)|internal/features/.*/infra(/|$)|\buserinfra\b|\bauthinfra\b' user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md
```

Run Go verification:

```bash
cd user-service
go test ./...
```

If implementation only touches user-service feature adapter imports and long-lived docs, `common/` tests are not required because `common` code is not in scope.

## Risks And Mitigations

### Missed imports

Risk: a package still imports `/infra`, causing compile failures or stale naming.

Mitigation: run `go test ./...` and targeted `rg` scans over `user-service` and current docs.

### Boundary drift

Risk: implementation moves application-owned ports or repository interfaces into `infrastructure` because the adapter directory is being renamed.

Mitigation: keep moves limited to current `infra/postgres` and `infra/redis` files. Leave `application/ports.go` untouched except for import path changes if required.

### Hidden behavior drift

Risk: while touching adapter files, implementation accidentally changes query semantics, Redis behavior or error mapping.

Mitigation: keep edits mechanical, review diff around non-import lines, and avoid unrelated refactors.

### Integration boundary confusion

Risk: contributors confuse feature-owned `infrastructure` with service-level `internal/integration`.

Mitigation: documentation should state that `infrastructure` is for this service's owned PostgreSQL/Redis adapters, while `internal/integration` is for external systems.
