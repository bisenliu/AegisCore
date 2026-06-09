## 1. Prepare Capability Package Structure

- [x] 1.1 Create target package directories under `user-services/internal/user`, `user-services/internal/user/api`, `user-services/internal/user/store/postgres`, `user-services/internal/auth`, `user-services/internal/auth/api`, and `user-services/internal/auth/store/redis`.
- [x] 1.2 Decide whether any existing type truly requires `user-services/internal/shared`; if none meets the documented review criteria, keep `internal/shared` absent.
- [x] 1.3 Confirm no Ent schema or migration change is needed; do not edit `user-services/ent/` generated files.

## 2. Migrate User Capability

- [x] 2.1 Move user HTTP DTOs and Swagger helper DTOs from `internal/api/user` to `internal/user/api`, preserving JSON/form/uri/validate/binding tags.
- [x] 2.2 Move user controller logic into `internal/user/controller.go` and update package imports while preserving route behavior and response envelopes.
- [x] 2.3 Move user service logic into `internal/user/service.go`, user model/errors/mapper into the user package, and define pure command/query types in `internal/user/commands.go`.
- [x] 2.4 Move user service consumer ports and input structs into `internal/user/ports.go`, keeping interfaces minimal and free of Ent/Redis/Gin/HTTP DTO types.
- [x] 2.5 Update user controller methods to map API request DTOs into user command/query values before calling service.
- [x] 2.6 Move PostgreSQL user repository implementation into `internal/user/store/postgres` and rename it as store implementation without changing persistence behavior.
- [x] 2.7 Extract Ent list predicate construction into `internal/user/store/postgres/predicates.go` and verify no user service file imports `user-services/ent/user` or `user-services/ent/predicate`.
- [x] 2.8 Move or update user tests to the new packages, preserving create/query/list success, validation error, not found, conflict, and internal error coverage.

## 3. Migrate Auth Capability

- [x] 3.1 Move auth HTTP DTOs from `internal/api/auth` to `internal/auth/api`, preserving JSON/validate/binding tags and response fields.
- [x] 3.2 Move auth controller logic into `internal/auth/controller.go` and update route references without changing auth HTTP paths.
- [x] 3.3 Move auth service, token, credential, session, redis key, command, model, and port definitions into `internal/auth`.
- [x] 3.4 Move Redis auth session repository into `internal/auth/store/redis` and keep Redis key format and TTL behavior unchanged.
- [x] 3.5 Update auth controller methods to map API request DTOs into auth command values before calling service.
- [x] 3.6 Move or update auth tests to the new packages, preserving login, password-change, refresh, logout, logout-all, token-version, and Redis session behavior coverage.

## 4. Rewire Runtime Integration

- [x] 4.1 Update `user-services/internal/bootstrap` provider imports, Fx annotations, optional dependencies, and constructor references for the new user/auth packages.
- [x] 4.2 Update `user-services/internal/router` route parameter types and imports for the new user/auth controllers.
- [x] 4.3 Update Swagger annotations or generated docs references that point to old `internal/api/*` DTO packages.
- [x] 4.4 Remove obsolete empty packages or files under `internal/controller`, `internal/service`, `internal/repository`, `internal/api`, and `internal/domain` after all imports are migrated.

## 5. Update Agent and Capability Documentation

- [x] 5.1 Update `AGENTS.md` repository shape and key entry points to show the target `internal/user` and `internal/auth` capability structure.
- [x] 5.2 Add formal AGENTS.md rules for `internal/shared` restrictions, ports ownership, validators purity, adapter thinness, request DTO to command mapping, and Ent predicate encapsulation.
- [x] 5.3 Include the requested `AuthUserAdapter` example and explain that field trimming is allowed while complex business orchestration in `adapter.go` is forbidden.
- [x] 5.4 Include the requested request DTO to command mapping example and explain that transport DTOs and application commands must not be mixed.
- [x] 5.5 Include the requested `store/postgres/predicates.go` example and an explicit anti-example forbidding `service.go` from importing Ent user predicates such as `user.StatusEQ`.
- [x] 5.6 Update `docs/opsx/CAPABILITY_MAP.md` code locations for affected capabilities after package migration.

## 6. Verification

- [x] 6.1 Run `gofmt -w` on changed Go files.
- [x] 6.2 Run `go test ./...` in `common/`.
- [x] 6.3 Run `go test ./...` in `user-services/`.
- [x] 6.4 Verify no service-layer files import `github.com/gin-gonic/gin`, `net/http`, `user-services/ent/user`, or `user-services/ent/predicate`.
- [x] 6.5 Verify HTTP API paths, response envelope fields, error codes, config keys, Redis key format, database schema, and migration files remain unchanged.
- [x] 6.6 Run `openspec status --change "standardize-project-structure-boundaries"` and confirm the change is apply-ready.
