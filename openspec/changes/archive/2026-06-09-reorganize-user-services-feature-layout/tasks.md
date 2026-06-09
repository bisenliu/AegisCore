## 1. Feature Directory Structure

- [x] 1.1 Create `user-services/internal/features/user/{api,app,domain,store/postgres}` and `user-services/internal/features/auth/{api,app,domain,store/postgres,store/redis}` directories.
- [x] 1.2 Move user HTTP DTO and Swagger document models from `internal/user/api` to `internal/features/user/api`.
- [x] 1.3 Move user controller, service, commands, ports, mapper and app-layer tests from `internal/user` to `internal/features/user/app`.
- [x] 1.4 Move user entity, status enum, domain errors and domain tests from `internal/user` to `internal/features/user/domain`.
- [x] 1.5 Move user Ent/PostgreSQL adapter and tests from `internal/user/store/postgres` to `internal/features/user/store/postgres`.
- [x] 1.6 Move auth HTTP DTO and Swagger document models from `internal/auth/api` to `internal/features/auth/api`.
- [x] 1.7 Move auth controller, service, commands, credentials, tokens, sessions, ports and app-layer tests from `internal/auth` to `internal/features/auth/app`.
- [x] 1.8 Move auth session/credential models, domain errors, Redis key semantics and domain tests from `internal/auth` to `internal/features/auth/domain`.
- [x] 1.9 Move auth Redis session adapter and tests from `internal/auth/store/redis` to `internal/features/auth/store/redis`.
- [x] 1.10 Add or move auth PostgreSQL credential/token-version adapter code into `internal/features/auth/store/postgres`, keeping credential and token-version persistence separate from user profile store behavior.

## 2. Import And Wiring Migration

- [x] 2.1 Update all moved files to use the new `package` names and imports for `features/user/{api,app,domain,store/postgres}`.
- [x] 2.2 Update all moved files to use the new `package` names and imports for `features/auth/{api,app,domain,store/postgres,store/redis}`.
- [x] 2.3 Update `user-services/internal/bootstrap` Fx providers to construct user profile app services/controllers, auth app services/controllers, user PostgreSQL store, auth PostgreSQL adapter and auth Redis store from the new feature paths.
- [x] 2.4 Update `user-services/internal/router` route registration to depend on the new user and auth app controller types without changing registered HTTP paths or auth middleware grouping.
- [x] 2.5 Update `user-services/internal/validators` imports so validators may depend on feature API DTOs and domain enums only, and still do not import Gin, Ent, Redis, app service or store packages.
- [x] 2.6 Update Swagger annotations and generated doc type references to use feature API packages while preserving existing OpenAPI paths, security metadata and response schemas.
- [x] 2.7 Remove obsolete empty `user-services/internal/user` and `user-services/internal/auth` package directories after all imports are migrated.

## 3. Behavior Preservation Checks

- [x] 3.1 Verify user create, query and list handlers still map HTTP DTOs to app commands/queries before calling services.
- [x] 3.2 Verify auth login, refresh, change-password, logout and logout-all handlers still map HTTP DTOs to app commands before calling services.
- [x] 3.3 Verify app services depend on consumer-owned ports and domain models, not Ent, Redis clients or concrete store packages.
- [x] 3.4 Verify store adapters map Ent/Redis persistence details to feature domain types and domain errors.
- [x] 3.5 Verify no new `internal/shared`, `internal/controller`, `internal/service`, `internal/repository`, `internal/api` or `internal/domain` packages are introduced.
- [x] 3.6 Verify Ent schema, generated `user-services/ent/` code, migrations, Redis key formats, YAML config keys and `AEGISCORE_` environment overrides are unchanged.

## 4. Documentation And Spec Alignment

- [x] 4.1 Update `AGENTS.md` to describe the new `internal/features` layout and feature-local `api/app/domain/store` responsibilities.
- [x] 4.2 Update `docs/ARCHITECTURE.md` runtime and request-flow path references to the new feature paths.
- [x] 4.3 Update `docs/opsx/CAPABILITY_MAP.md` capability-to-code mappings for user, auth, runtime, validation, response and Swagger references.
- [x] 4.4 Update stable OpenSpec specs that mention old `internal/user`, `internal/auth`, `internal/service`, `internal/repository`, `internal/dto` or `internal/domain` paths so they align with `user-domain-boundary`.
- [x] 4.5 Confirm docs do not describe the old ability-root layout as the current target structure.

## 5. Formatting And Tests

- [x] 5.1 Run `gofmt` on all moved or edited Go files.
- [x] 5.2 Run `rg "github.com/aegiscore/user-services/internal/(user|auth|service|repository|dto|domain)" user-services docs openspec` and resolve stale references except historical archived changes.
- [x] 5.3 In `user-services/`, run `go test ./...` and fix compile or behavior regressions.
- [x] 5.4 If Swagger docs are regenerated during implementation, verify `user-services/docs/` changes only reflect package path/type reference updates and not API contract drift.
- [x] 5.5 Record that this refactor does not require `go generate ./ent`, Atlas migration generation or dependency updates because schema, generated code and module requirements remain unchanged.
