## 1. Router Probe Aggregation

- [x] 1.1 Extend the router-facing `HealthChecker` contract with a stable component name if needed for timeout fallback reporting.
- [x] 1.2 Update `probez` to run non-nil readiness/startup checkers concurrently under the existing shared 500ms request context.
- [x] 1.3 Preserve deterministic response ordering by original checker order and continue skipping nil checkers.
- [x] 1.4 Normalize empty checker result status to `unavailable` and fill empty result names from the checker name.
- [x] 1.5 Return unavailable timeout results for pending checkers when the shared context expires before all checks complete.

## 2. Provider Checkers

- [x] 2.1 Add stable `Name()` methods to PostgreSQL, Redis, Casbin policy, and RBAC policy watcher health checkers if the interface is extended.
- [x] 2.2 Keep checker names unchanged: `postgres.user_db`, `redis.cache_redis`, `rbac.casbin_policy`, and `rbac.policy_watcher`.
- [x] 2.3 Keep provider wiring and readiness/startup dependency membership unchanged.

## 3. Tests

- [x] 3.1 Update existing router health test fakes to satisfy the final `HealthChecker` contract.
- [x] 3.2 Add a concurrency regression test proving two slow checks can complete successfully within one shared probe budget.
- [x] 3.3 Add a timeout aggregation test proving a pending checker is reported as unavailable with a stable component name.
- [x] 3.4 Preserve existing success and failure response tests for `/livez`, `/readyz`, and `/startupz`.
- [x] 3.5 Update provider tests or compile-time checks for any health checker interface changes.

## 4. Documentation And Verification

- [x] 4.1 Update the existing health probe design notes to describe concurrent readiness/startup aggregation under one request budget.
- [x] 4.2 Run `gofmt` on touched Go files.
- [x] 4.3 Run `go test ./...` under `user-service/`.
- [x] 4.4 Inspect the final diff to confirm no business API contracts, RBAC semantics, generated Ent code, migrations, Redis key contracts, or deployment probe thresholds changed accidentally.
