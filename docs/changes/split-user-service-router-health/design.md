# Design

## Overview

This change is a router package cleanup inside `user-service`. It does not alter runtime wiring or HTTP behavior.

Target ownership:

- `user-service/internal/router/router.go`: route params, top-level route registration, and `/api/v1` feature grouping.
- `user-service/internal/router/health.go`: `/healthz` response DTO, handler, and health route registration.
- `user-service/internal/router/swagger.go`: Swagger UI, `/docs`, `/api-docs`, and Swagger enablement rules.

The provider boundary remains unchanged:

```text
providers/routes.go
  -> router.RegisterUserServiceHTTPRoutes
  -> registerHealthRoutes
  -> RegisterSwagger
  -> registerV1Routes
```

`internal/providers` continues adapting Fx dependencies into `router.RouteParams`; `internal/router` continues defining HTTP route structure without importing Fx.

## Target Router Layout

```text
user-service/internal/router/
  router.go
  health.go
  health_test.go
  swagger.go
  swagger_test.go
```

`health_test.go` may be omitted if existing provider route tests already cover the contract thoroughly, but a focused router-level test is preferred because the health route is owned by `router`.

`system.go` should be removed after its contents move to `health.go`. Keeping a generic system-route file would preserve the ambiguous boundary this change is meant to remove.

## router.go

`router.go` should keep:

- `RouteParams`.
- `RegisterUserServiceHTTPRoutes`.
- `registerV1Routes`.
- Imports needed for common auth middleware, config, auth/user feature HTTP routes, Gin, and Zap.

`RegisterUserServiceHTTPRoutes` should call the health and Swagger registrars in a small, readable sequence:

```go
func RegisterUserServiceHTTPRoutes(engine *gin.Engine, params RouteParams) {
    registerHealthRoutes(engine, params.ServiceName)
    RegisterSwagger(engine, params.Environment)
    registerV1Routes(engine, params)
}
```

Do not move feature route grouping out of `router.go` as part of this change. The purpose is to separate system health and Swagger files, not to redesign all router registration.

## health.go

Move the current health code from `system.go` into `health.go`:

- `const healthStatusOK = "ok"`
- `type HealthResponse struct`
- route registrar
- `healthz(serviceName string) gin.HandlerFunc`
- Swagger annotations for `/healthz`

Use an explicit registrar name:

```go
func registerHealthRoutes(engine *gin.Engine, serviceName string) {
    engine.GET("/healthz", healthz(serviceName))
}
```

This keeps the registration private to the router package and mirrors the existing private `registerV1Routes` helper. There is no need to export health registration unless another package has a real consumer.

The health response must remain:

```json
{
  "status": "ok",
  "service": "<configured service name>"
}
```

with HTTP 200.

Do not add readiness, dependency probing, timestamps, versions, build info, uptime, database checks, Redis checks, or Ent checks.

## swagger.go

Keep Swagger code in `swagger.go`:

- `swaggerEnabledEnv`.
- `RegisterSwagger`.
- `redirectToSwagger`.
- `swaggerEnabled`.

No behavior change is intended:

- production-like environments disable Swagger by default.
- `SWAGGER_ENABLED` overrides the environment default when it parses as a bool.
- invalid `SWAGGER_ENABLED` values fall back to environment defaults.
- `/docs` and `/api-docs` redirect to `/swagger/index.html`.

Do not move Swagger registration into `router.go` beyond the existing call from `RegisterUserServiceHTTPRoutes`.

## Tests

Add or adjust focused router tests:

- `TestRegisterHealthRoutes` or similar:
  - create a fresh Gin engine.
  - call `registerHealthRoutes(engine, "aegiscore-user-services")`.
  - request `GET /healthz`.
  - assert HTTP 200.
  - assert JSON response has `status: "ok"` and `service: "aegiscore-user-services"`.

- `TestRegisterUserServiceHTTPRoutesIncludesHealthAndSwagger` can remain in `internal/providers/routes_test.go` if it already verifies provider integration with `/healthz` and Swagger. Keep it passing after renaming `registerSystemRoutes` to `registerHealthRoutes`.

- Existing `swagger_test.go` should continue covering:
  - production default disabled.
  - local/default enabled.
  - `SWAGGER_ENABLED=true/false` override.
  - `/swagger/index.html` success when enabled.
  - `/docs` and `/api-docs` redirects.
  - disabled Swagger returns 404.

Use `gin.SetMode(gin.TestMode)` in tests when needed. Keep tests deterministic and independent of external PostgreSQL or Redis.

## Documentation Updates

Update long-lived docs:

- `AGENTS.md`
  - Add `user-service/internal/router/health.go` to Key Entry Points.
  - Clarify that `router.go` is the route graph entrypoint and `health.go` owns `/healthz`.
  - Ensure Swagger entry remains `user-service/internal/router/swagger.go`.

- `docs/ARCHITECTURE.md`
  - Update HTTP Request Flow row for route assembly to mention `router.go`, `health.go`, and `swagger.go`.
  - Update runtime flow wording if it describes all system routes as living in `router.go`.

No `docs/DEVELOPMENT.md` update is required unless implementation finds an active statement that names `system.go` or says `router.go` owns the health handler.

Do not reintroduce OpenSpec/OPSX references or artifacts.

## Compatibility

This is an internal file split in a Go package. Because `system.go`, `health.go`, `router.go`, and `swagger.go` all share `package router`, moving private symbols does not change the public import path.

The only externally visible type involved is `HealthResponse`. Its name and fields should remain stable so generated Swagger references and tests do not drift unnecessarily.

The Swagger annotations for `/healthz` should move with the handler. If generated Swagger docs are checked in and a later implementation runs `make swagger-generate`, the output should not contain semantic changes for `/healthz`.

## Verification Strategy

After implementation, run:

```bash
rg -n "registerSystemRoutes|system.go" user-service/internal/router AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md
```

Expected result: no active references to the old system route helper or file.

Run:

```bash
rg -n "registerHealthRoutes|func healthz|type HealthResponse|RegisterSwagger" user-service/internal/router
```

Expected result:

- health symbols are in `health.go`.
- Swagger symbols are in `swagger.go`.
- `router.go` calls the health and Swagger registration helpers but does not define their handlers.

Run focused tests:

```bash
go test ./internal/router ./internal/providers
```

from `user-service/`.

If the change touches docs or imports only, broader `go test ./...` in `user-service/` is still useful before merge, but the acceptance gate for this change is health and Swagger route coverage.
