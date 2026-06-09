## 1. Prepare Target Feature Layout

- [x] 1.1 Create `user-services/internal/features/user/{transport/http,infra/postgres}` and `user-services/internal/features/auth/{transport/http,infra/postgres,infra/redis}` directories.
- [x] 1.2 Add `user-services/internal/features/user/module.go` and `user-services/internal/features/auth/module.go` as feature Fx module entrypoints.
- [x] 1.3 Confirm no Ent schema, generated `user-services/ent/` code, Atlas migration, Go module, or external dependency change is needed.

## 2. Migrate User Feature

- [x] 2.1 Move user HTTP controller and controller tests from `features/user/app` to `features/user/transport/http`.
- [x] 2.2 Move user HTTP validation/normalization from `internal/validators/user.go` and tests to `features/user/transport/http/validation.go` and matching tests.
- [x] 2.3 Add user `transport/http/routes.go` with `RegisterRoutes(group *gin.RouterGroup, controller *Controller)` for `GET /:user_id`, `GET /`, and `POST /`.
- [x] 2.4 Move user PostgreSQL adapter and tests from `features/user/store/postgres` to `features/user/infra/postgres`.
- [x] 2.5 Update user package imports so `app` depends only on domain, ports, commands/queries, mapper, and common non-HTTP primitives.
- [x] 2.6 Verify user `app` package does not import Gin, `common/http/ginvalidation`, HTTP response helpers, Ent, Redis, or concrete infra packages.

## 3. Migrate Auth Feature

- [x] 3.1 Move auth HTTP controller and controller tests from `features/auth/app` to `features/auth/transport/http`.
- [x] 3.2 Move auth HTTP validation/normalization from `internal/validators/auth.go` and tests to `features/auth/transport/http/validation.go` and matching tests.
- [x] 3.3 Add auth `transport/http/routes.go` with `RegisterPublicRoutes` and `RegisterProtectedRoutes`.
- [x] 3.4 Move auth PostgreSQL credential/token-version adapter and tests from `features/auth/store/postgres` to `features/auth/infra/postgres`.
- [x] 3.5 Move auth Redis session adapter, Redis key helper placement as needed, and tests from `features/auth/store/redis` to `features/auth/infra/redis`, preserving Redis key format and TTL behavior.
- [x] 3.6 Update auth package imports so `app` depends on domain, ports, credential/token/session components, and common security primitives, not Gin, Ent, Redis clients, or HTTP response writers.

## 4. Rewire Fx Modules And Runtime Routes

- [x] 4.1 Move user app service, user HTTP controller, and user infra provider wiring from `bootstrap.AppModule` into `features/user.Module`.
- [x] 4.2 Move auth app service, auth HTTP controller, auth PostgreSQL infra, auth Redis infra, and token-version validator provider wiring from `bootstrap.AppModule` into `features/auth.Module`.
- [x] 4.3 Update `bootstrap.AppModule` to import `user.Module` and `auth.Module` while keeping shared config, Zap logger, timezone, validation, named PostgreSQL/Redis runtime, Ent clients, Gin engine, HTTP server, and route invoke in bootstrap.
- [x] 4.4 Update service-level route total assembly to create `/api/v1`, public auth, protected auth, and protected user groups, then call feature-local route registration functions.
- [x] 4.5 Keep health and Swagger route registration in service-level runtime/router code and preserve existing Swagger enabled behavior.
- [x] 4.6 Remove obsolete user/auth provider imports and any no-longer-used `internal/validators` package files or references.

## 5. Preserve Behavior And Boundaries

- [x] 5.1 Verify user create, query, and list controllers still bind API DTOs, run feature-local HTTP validation, map to app command/query, and return `common/contract/response.Envelope`.
- [x] 5.2 Verify auth login, refresh, change-password, logout-current, and logout-all controllers still bind API DTOs, run feature-local HTTP validation, map to app command, and preserve public/protected route grouping.
- [x] 5.3 Verify app services still own business orchestration, error mapping, password hashing, token/session logic, pagination normalization, and persistence-dependent validation.
- [x] 5.4 Verify infra adapters map Ent/Redis persistence details to feature domain types and domain errors without importing Gin or response helpers.
- [x] 5.5 Verify `features/user/infra/postgres/predicates.go` owns Ent predicate construction and no app service imports `user-services/ent/user` or `user-services/ent/predicate`.
- [x] 5.6 Verify no new `internal/shared`, `internal/controller`, `internal/service`, `internal/repository`, `internal/api`, or `internal/domain` packages are introduced.

## 6. Documentation And Spec Alignment

- [x] 6.1 Update `AGENTS.md` repository shape, key entry points, final layout, dependency matrix, route ownership, module ownership, validation placement, and infra naming rules.
- [x] 6.2 Update `docs/ARCHITECTURE.md` runtime flow, request flow, feature organization, and data access path references.
- [x] 6.3 Update `docs/opsx/CAPABILITY_MAP.md` code locations for user query/create/list, auth session control, HTTP runtime, request validation, response contract, and Swagger documentation.
- [x] 6.4 Update non-archive OpenSpec specs that still reference `features/*/app/controller.go`, `features/*/store/*`, or `user-services/internal/validators`.
- [x] 6.5 Confirm archived changes remain untouched except for this new change directory.

## 7. Formatting And Verification

- [x] 7.1 Run `gofmt -w` on moved or edited Go files.
- [x] 7.2 Run `rg "features/.*/app/controller|features/.*/store|internal/validators" AGENTS.md docs openspec/specs user-services/internal` and resolve stale non-archive references.
- [x] 7.3 Run dependency checks to confirm `features/*/app` does not import Gin, Ent, Redis clients, HTTP binder, or response writer packages.
- [x] 7.4 In `user-services/`, run `go test ./...` and fix compile or behavior regressions.
- [x] 7.5 Run `go test ./...` in `common/` if common files changed; otherwise record that common was not modified.
- [x] 7.6 If Swagger docs are regenerated, verify generated docs only reflect package path/type reference updates and no API contract drift.
- [x] 7.7 Run `openspec status --change "standardize-feature-module-layout"` and confirm the change is apply-ready.
