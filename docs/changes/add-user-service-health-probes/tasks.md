## 1. Probe Contract And Router

- [x] 1.1 Add router-level health check result types and a narrow `HealthChecker` interface without importing SQL, Redis, Ent, or permission infrastructure into `internal/router`.
- [x] 1.2 Register `GET /livez`, `GET /readyz`, and `GET /startupz` from `registerHealthRoutes`.
- [x] 1.3 Remove the legacy `GET /healthz` compatibility alias and keep liveness on `GET /livez`.
- [x] 1.4 Implement readiness/startup response aggregation with HTTP 200 when all checks pass and HTTP 503 when any check fails.
- [x] 1.5 Add Swagger annotations for the new probe endpoints.

## 2. Service-Level Health Check Providers

- [x] 2.1 Add `user-service/internal/providers/health.go` to build health checkers from named PostgreSQL, Redis, Casbin, and RBAC watcher dependencies.
- [x] 2.2 Add a PostgreSQL `user_db` checker using bounded `PingContext`.
- [x] 2.3 Add a Redis `cache_redis` checker using bounded `Ping`.
- [x] 2.4 Add a Casbin policy checker that reports unavailable when the permission Casbin engine `LastError()` is non-nil.
- [x] 2.5 Add an RBAC policy watcher checker that reports unavailable when the watcher is not running or has a last unexpected error.
- [x] 2.6 Wire the health check collection through `providers.RegisterRoutes` into `router.RouteParams`.

## 3. RBAC Watcher Status

- [x] 3.1 Extend `permission/infrastructure/redis.Watcher` with concurrency-safe running status.
- [x] 3.2 Track the latest unexpected watcher loop or subscription error without treating normal shutdown cancellation as an error.
- [x] 3.3 Expose the watcher status through a minimal interface consumed by provider health checks.
- [x] 3.4 Preserve existing RBAC policy sync behavior and English log message conventions.

## 4. Tests

- [x] 4.1 Update `user-service/internal/router/health_test.go` to cover `/livez`, `/readyz`, and `/startupz`.
- [x] 4.2 Update `user-service/internal/providers/routes_test.go` to expect the new probe routes in route registration.
- [x] 4.3 Add provider health checker tests for PostgreSQL, Redis, Casbin LastError, and watcher status mapping.
- [x] 4.4 Extend RBAC watcher tests to cover running status and last error behavior.
- [x] 4.5 Run `go test ./...` under `user-service/`.

## 5. Docs And Generated Artifacts

- [x] 5.1 Update `docs/ARCHITECTURE.md` route flow text to mention `/livez`, `/readyz`, and `/startupz`.
- [x] 5.2 Update `docs/PRODUCT.md` or deployment guidance that currently treats `/healthz` as the operational health endpoint.
- [x] 5.3 Update Kubernetes or Helm probe examples if existing deployment assets define or document probe paths.
- [x] 5.4 Regenerate Swagger docs if route annotations changed.
- [x] 5.5 Inspect the final diff to confirm no Ent generated code, migration files, business API contracts, or RBAC authorization semantics changed accidentally.
