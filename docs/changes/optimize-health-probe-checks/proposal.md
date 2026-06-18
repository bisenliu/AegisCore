## Why

`/readyz` and `/startupz` currently execute PostgreSQL, Redis, Casbin policy, and RBAC policy watcher checks sequentially under one shared 500ms probe context. PostgreSQL and Redis both consume that same deadline for network I/O. If the first dependency is slow but still healthy, later dependencies receive only the remaining time and can fail with `context deadline exceeded` even though they would have passed with a full probe budget.

This creates a readiness false negative risk during transient datastore or network jitter. In Kubernetes-style traffic control, repeated readiness failures can remove an otherwise usable pod from service endpoints and reduce availability. The liveness endpoint is already isolated from dependency checks, so the optimization should focus on readiness/startup aggregation without changing liveness semantics.

## What Changes

- Run `/readyz` and `/startupz` dependency checks concurrently instead of serially.
- Keep the existing bounded per-request probe budget so dependency stalls do not hold Gin request workers indefinitely.
- Preserve the current response shape, status values, route paths, and dependency set.
- Preserve deterministic response ordering by returning component results in configured checker order.
- Report timed-out or non-returning checks as unavailable with a stable, non-sensitive message.
- Add focused router tests that reproduce the cumulative timeout problem and prove the checks now run within the shared probe wall-clock budget.
- Update the health probe design notes to document concurrent aggregation and timeout behavior.

## Capabilities

### Updated Capabilities

- `user-service-health-probes`: Readiness and startup probes continue to validate PostgreSQL, Redis, Casbin policy, and RBAC policy watcher health, but dependency checks are aggregated concurrently to reduce false negatives caused by cumulative timeout consumption.

### Removed Capabilities

- None.

## Impact

- Affects `user-service/internal/router/health.go` readiness/startup check aggregation.
- May require a narrow extension to the router-facing health checker contract, such as a stable checker name, so timeout results can identify non-returning components.
- Affects router health tests and possibly provider health checker tests if the interface changes.
- Affects documentation under `docs/changes/add-user-service-health-probes/` or current architecture/development notes that describe probe timeout behavior.
- Does not change business APIs, RBAC authorization semantics, datastore schemas, Ent schema, Atlas migrations, Redis key contracts, route paths, or OpenAPI response contracts.
