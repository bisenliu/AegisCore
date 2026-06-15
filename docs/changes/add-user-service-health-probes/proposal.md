## Why

`/healthz` currently always returns HTTP 200 and can only prove that the Gin process can answer a request. Kubernetes or another traffic controller can therefore route traffic to a pod before the user-service has confirmed PostgreSQL, Redis, the in-memory Casbin policy, and the RBAC policy watcher are ready.

This is risky during rolling deployments and dependency recovery. A pod that has not connected to required datastores, failed to load RBAC policy, or lost its policy watcher can still be added to service endpoints, causing request failures or fail-closed authorization behavior on normal business routes.

## What Changes

- Add separate probe routes:
  - `GET /livez` for process liveness.
  - `GET /readyz` for traffic readiness.
  - `GET /startupz` for startup completion.
- Add a router-level health dependency abstraction so health routes can check runtime dependencies without importing Ent, Redis clients, SQL details, or feature infrastructure directly.
- Add service-level readiness check providers for:
  - PostgreSQL `user_db` ping.
  - Redis `cache_redis` ping.
  - Casbin engine `LastError`.
  - RBAC policy watcher running state and last loop error.
- Return HTTP 200 only when the selected probe is healthy; return HTTP 503 with a structured component summary when a readiness or startup dependency is not ready.
- Update tests, Swagger docs, product/development documentation, and deployment probe examples so operations use `/livez`, `/readyz`, and `/startupz` instead of treating `/healthz` as readiness.

## Capabilities

### New Capabilities

- `user-service-health-probes`: Kubernetes-style liveness, readiness, and startup probe endpoints for user-service runtime dependencies.

### Removed Capabilities

- `user-service-health`: Remove the legacy `/healthz` endpoint instead of keeping a compatibility alias; callers must use `/livez`, `/readyz`, or `/startupz` explicitly.

## Impact

- Affects `user-service/internal/router/health.go` and route registration tests.
- Affects `user-service/internal/providers` by adding health dependency provider wiring.
- Affects permission RBAC infrastructure only to expose watcher status without moving RBAC policy ownership out of the permission feature.
- Affects Swagger generated docs and product/deployment documentation for probe paths.
- Does not change business API contracts, authorization semantics, datastore schemas, Ent schema, Atlas migrations, Redis key contracts, or RBAC policy source data.
