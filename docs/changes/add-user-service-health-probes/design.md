## Context

The user-service route graph currently registers `/healthz` in `user-service/internal/router/health.go`. The handler returns a fixed status and service name, so it is suitable only as a liveness signal.

The service already fails fast during Fx startup if required PostgreSQL or Redis ping checks fail, and the Casbin engine records its most recent load or reload error through `LastError()`. The RBAC policy watcher is started through Fx lifecycle and is critical to distributed policy freshness after startup. Those existing signals should be surfaced through readiness/startup probes without moving service-specific dependency logic into `common`.

Repository constraints require health route ownership to remain in `internal/router`, service-level dependency assembly to remain in `internal/providers`, RBAC authorization ownership to remain in the permission feature, and comments in Go source to remain Chinese while log messages remain English.

## Goals / Non-Goals

**Goals:**

- Provide `/livez`, `/readyz`, and `/startupz` with Kubernetes-compatible status code behavior.
- Remove `/healthz` instead of preserving a backward-compatible liveness alias.
- Make readiness fail when PostgreSQL, Redis, Casbin policy load state, or the RBAC policy watcher are unhealthy.
- Make startup fail until the same critical startup dependencies are ready.
- Keep probe responses small, stable, and free of secrets.
- Keep health check dependency interfaces narrow and owned by the router/provider boundary.
- Add focused tests for success and failure behavior.
- Update docs and checked-in Swagger artifacts when probe annotations change.

**Non-Goals:**

- No new database tables, Redis keys, Ent schema changes, or Atlas migrations.
- No change to business route authorization behavior.
- No request-time Redis strong consistency check for RBAC authorization.
- No new generic `common/runtime/health` framework unless a later multi-service need justifies it.
- No broad Kubernetes or Helm redesign beyond probe path updates or examples.
- No compatibility alias for `/healthz`.

## Decisions

### Probe semantics

Use the following endpoint meanings:

| Endpoint | Healthy status | Failure status | Semantics |
|---|---:|---:|---|
| `/livez` | 200 | normally none | Process is alive and Gin can answer requests. |
| `/readyz` | 200 | 503 | Pod can safely receive traffic. |
| `/startupz` | 200 | 503 | Pod completed critical runtime initialization. |

`/livez` should avoid dependency checks so temporary PostgreSQL, Redis, or policy reload failures do not trigger liveness restarts. `/readyz` and `/startupz` share the same critical dependency set initially because all listed dependencies are required before safe traffic admission.

### Router-owned response contract

Add a health response model in `user-service/internal/router/health.go` similar to:

```json
{
  "status": "ok",
  "service": "aegiscore-user-services",
  "checks": [
    {"name": "postgres.user_db", "status": "ok"},
    {"name": "redis.cache_redis", "status": "ok"},
    {"name": "rbac.casbin_policy", "status": "ok"},
    {"name": "rbac.policy_watcher", "status": "ok"}
  ]
}
```

On failure, set top-level `status` to `unavailable`, return HTTP 503, and include component entries with a concise `message`. Messages must not include DSNs, Redis addresses with credentials, tokens, cookies, raw request bodies, SQL strings, or stack traces.

### Narrow health check interface

Define small router-facing types in `health.go` or a nearby router file:

- `HealthChecker` with `Check(ctx context.Context) HealthCheckResult`.
- `HealthCheckResult` with `Name`, `Status`, and optional `Message`.
- `HealthCheckStatus` constants for `ok` and `unavailable`.

`RouteParams` should accept a `HealthChecks` value or separate `ReadinessChecks` and `StartupChecks` slices. The router consumes only the interface, not concrete SQL/Redis/Casbin types.

### Provider-owned dependency checks

Add provider wiring under `user-service/internal/providers`, for example `health.go`, to construct health checkers from service-level resources:

- PostgreSQL checker receives named `*sql.DB` `user_db` and calls `PingContext` with a short timeout.
- Redis checker receives named `*redis.Client` `cache_redis` and calls `Ping`.
- Casbin checker receives the permission Casbin engine through a small provider adapter and fails if `LastError()` is non-nil.
- RBAC watcher checker receives watcher status through a permission-owned status interface and fails if the watcher is not running or has recorded its latest loop/subscription error.

The provider file is allowed to depend on service resources and permission feature interfaces because it is service-level assembly. The router remains decoupled from these implementation details.

### RBAC watcher status

Extend `user-service/internal/features/permission/infrastructure/redis.Watcher` with concurrency-safe status methods. A minimal shape is:

- `Running() bool`
- `LastError() error`

Record subscription setup failures and unexpected Pub/Sub loop termination as last errors. Do not treat normal context cancellation during shutdown as an unhealthy error. The watcher should continue to log English messages with stable snake_case fields.

This keeps watcher mechanics in permission Redis infrastructure while letting provider-level health checks observe whether the background policy sync path is active.

### Startup readiness source

Fx startup already orders datastore pings, Casbin engine construction, watcher startup, route registration, and HTTP server lifecycle. `/startupz` can therefore use the same dependency checkers as readiness. If a later change introduces long-running warmup state, it can add a startup-only checker without changing `/readyz`.

### Timeouts and cancellation

Probe handlers should derive a bounded child context from `c.Request.Context()` for dependency checks. Start with a conservative per-probe timeout such as 500ms to avoid tying up request workers during dependency stalls. Readiness and startup dependency checks should run concurrently under that same per-request context so one slow but healthy dependency does not consume the deadline needed by later checks. Late or non-returning checks should be reported as unavailable with a stable timeout message and without exposing dependency internals.

### Documentation and deployment guidance

Update documentation that currently says `/healthz` is the operational health endpoint. Guidance should identify `/livez` for liveness, `/readyz` for readiness, and `/startupz` for startup probes. If Kubernetes or Helm manifests include probes now or are added in the same implementation, they should use those paths.

## Risks / Trade-offs

- Readiness now depends on external services, so transient PostgreSQL or Redis issues can remove pods from endpoints. This is intentional for traffic safety but should not restart pods through liveness.
- Casbin `LastError` can stay set after a failed reload until a later successful reload clears it, so readiness can reflect stale or failed RBAC policy state.
- RBAC watcher health adds observability for distributed policy freshness, but a process can still serve existing in-memory policy while watcher recovery is pending.
- Probe checks can add load to PostgreSQL and Redis if polled too frequently. The deployment configuration should use reasonable periods and timeouts.
- Removing `/healthz` is an intentional breaking change for old monitors; operations must use the explicit probe paths.

## Test Plan

- Unit test `/livez` returns HTTP 200 with service name and no dependency requirement.
- Unit test `/readyz` returns HTTP 200 when all checkers are healthy and includes component summaries.
- Unit test `/readyz` returns HTTP 503 when any checker is unavailable and identifies the failed component.
- Unit test `/startupz` uses startup checkers and mirrors critical dependency failure behavior.
- Provider tests cover PostgreSQL, Redis, Casbin LastError, and watcher status checker mapping.
- Watcher tests cover `Running()` and `LastError()` state during start, stop, subscription failure, and normal cancellation where practical.
- Run `go test ./...` in `user-service/`.
- Run Swagger generation if annotations or checked-in docs change.
