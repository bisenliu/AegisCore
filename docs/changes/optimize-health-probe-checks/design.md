## Context

`user-service/internal/router/health.go` owns `/livez`, `/readyz`, and `/startupz`. `probez` currently creates one `context.WithTimeout(c.Request.Context(), 500*time.Millisecond)` and then loops through configured health checkers one at a time. `user-service/internal/providers/health.go` wires the readiness/startup checker list as PostgreSQL `user_db`, Redis `cache_redis`, Casbin policy state, and RBAC policy watcher state.

The PostgreSQL checker calls `PingContext(ctx)` and the Redis checker calls `Ping(ctx).Err()` using that shared context. Casbin and watcher checks are local state reads, so the availability risk comes mostly from serial network I/O consuming the same deadline. The existing design correctly avoids dependency checks in `/livez`; this change keeps that separation.

## Goals / Non-Goals

**Goals:**

- Reduce readiness/startup false negatives caused by cumulative serial timeout consumption.
- Keep one bounded wall-clock budget for a probe request.
- Execute non-nil dependency checks concurrently.
- Preserve deterministic output ordering matching configured checker order.
- Preserve existing `HealthResponse`, `HealthCheckResult`, status constants, HTTP status behavior, and route paths.
- Keep router decoupled from SQL, Redis, Ent, Casbin concrete implementation details, and permission infrastructure.
- Add focused tests for concurrency, timeout handling, nil checker handling, and existing success/failure behavior.

**Non-Goals:**

- No change to `/livez` behavior.
- No new generic `common/runtime/health` framework.
- No change to deployment probe thresholds, Kubernetes manifests, Helm charts, or Compose healthcheck configuration in this change.
- No new runtime metrics or tracing behavior.
- No change to the set of readiness/startup dependencies.
- No change to business API contracts or authorization behavior.

## Decisions

### Concurrent probe aggregation

`probez` should still create one bounded child context from the incoming request. Instead of running checks in a serial loop, it should submit each non-nil checker in its own goroutine and collect results through a buffered channel sized to the number of submitted checks.

The handler waits until either every submitted check returns or the shared context is done. When all checks return, it computes the top-level status exactly as today. When the context is done before all checks return, it records unavailable results for each pending checker and returns HTTP 503.

### Stable component names

Timeout fallback needs to identify which component did not return. The preferred implementation is to extend the router-facing `HealthChecker` interface with a stable `Name() string` method:

```go
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) HealthCheckResult
}
```

Each provider checker returns the same names it already writes into `HealthCheckResult`: `postgres.user_db`, `redis.cache_redis`, `rbac.casbin_policy`, and `rbac.policy_watcher`. Test fakes should implement `Name()` as well. This keeps timeout messages useful without importing concrete provider types into the router.

If an implementation returns an empty result name, the aggregator should fill it from `checker.Name()`. If the result status is empty, keep the current behavior and normalize it to `unavailable`.

### Ordering and nil handling

The output should remain deterministic. Store each returned result by its original checker index, then compact skipped nil entries out of the final slice. Nil checkers should continue to be ignored and should not consume a goroutine or create a placeholder result.

### Timeout messages

When a checker does not return before the shared context deadline, return:

- `Name`: the checker name.
- `Status`: `unavailable`.
- `Message`: a stable message such as `health check timeout`.

Do not include dependency addresses, DSNs, Redis URLs, raw errors, stack traces, SQL, tokens, cookies, request bodies, or other sensitive data.

### Goroutine behavior

The checker goroutines receive the shared context. PostgreSQL and Redis checks should return when the context is canceled. Local state checks should return immediately. The result channel must be buffered so a late-returning goroutine can send without blocking after the handler has already timed out.

The implementation should avoid adding a package-level worker pool or long-lived goroutines. Probe requests are short-lived and the checker count is fixed and small.

### Documentation

The original health probe design notes said individual checkers can use the same context rather than introducing independent global goroutines. Update that documentation to clarify that the shared context is still the per-request wall-clock budget, but the dependency checks run concurrently under it.

## Risks / Trade-offs

- Concurrent checks create a few short-lived goroutines per readiness/startup request. The checker list is currently four items, so this is an acceptable trade-off for lower false negative risk.
- If a dependency client ignores context cancellation, a checker goroutine could outlive the HTTP response until the client returns. The current PostgreSQL and Redis calls are context-aware, which keeps this bounded in expected operation.
- Extending `HealthChecker` with `Name()` touches test fakes and all provider checker implementations, but it keeps timeout reporting simple and explicit.
- The overall probe budget remains 500ms. A real dependency outage or sustained latency above that budget will still correctly return unavailable.

## Test Plan

- Add a router test with two blocking or delayed checkers that each complete within 500ms when run concurrently but would exceed 500ms if run serially; assert `/readyz` returns HTTP 200.
- Add a router test where one checker blocks until context cancellation; assert `/readyz` returns HTTP 503 and includes that checker name with `health check timeout`.
- Preserve existing tests for `/livez`, successful `/readyz`, successful `/startupz`, and dependency failure aggregation.
- Update provider health checker tests or compile-time coverage for the `Name()` method if the interface changes.
- Run `go test ./...` under `user-service/`.
- Run broader `make test-user-service` or `make test` if the touched surface expands beyond router/provider code.
