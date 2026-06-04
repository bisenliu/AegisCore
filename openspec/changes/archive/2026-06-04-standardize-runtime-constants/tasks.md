## 1. Shutdown Timeout Refactor

- [x] 1.1 Rename CLI lifecycle timeout constants in `user-services/cmd/main.go` so start timeout and Fx app stop timeout names clearly describe app lifecycle budgets.
- [x] 1.2 Adjust the default Fx app stop timeout so it is greater than or equal to `user-services/configs/config.yaml` `http.shutdown_timeout` and cannot truncate the default HTTP graceful shutdown budget.
- [x] 1.3 Rename or document the HTTP server fallback shutdown timeout in `user-services/internal/bootstrap/server.go` so it clearly applies only to HTTP graceful shutdown.
- [x] 1.4 Add or update tests covering the relationship between CLI/Fx stop budget, configured HTTP shutdown timeout, and HTTP fallback shutdown timeout.

## 2. Constants Ownership Review

- [x] 2.1 Review runtime constants across `common/` and `user-services/`, excluding generated `user-services/ent/` code except `user-services/ent/schema/`.
- [x] 2.2 Classify constants into cross-module contract, service runtime, business rule, data/schema rule, repository key format, example/documentation, and test scenario categories.
- [x] 2.3 Keep cross-module contract constants in owning packages such as `common/auth`, `common/response`, `common/middleware`, `common/config`, and `common/infrastructure`.
- [x] 2.4 Keep local implementation constants, format strings, one-off test values, and Swagger examples near their usage unless they duplicate a contract value that needs protection.

## 3. Duplicate Defaults And Business Thresholds

- [x] 3.1 Review auth token TTL fallback values against `user-services/configs/config.yaml` and DTO examples, then align values or document intentional differences.
- [x] 3.2 Review Redis auth session TTL and key format constants in `user-services/internal/repository/redis` and keep them owned by the Redis session repository boundary.
- [x] 3.3 Review user status defaults across DTO, service, and Ent schema, then either share a domain-owned source or add tests preventing drift.
- [x] 3.4 Review username and nickname length limits across DTO validation and Ent schema, then either share constants without creating bad dependencies or add consistency tests.
- [x] 3.5 Review service name, route path, Swagger annotation, config path, resource name, trace-id, response code, and migration path constants for actionable duplication versus acceptable local literals.

## 4. Verification

- [x] 4.1 Run `gofmt -w` on changed Go files.
- [x] 4.2 Run `go test ./...` in `common/`.
- [x] 4.3 Run `go test ./...` in `user-services/`.
- [x] 4.4 If any Ent schema file changes, run `go generate ./ent` in `user-services/` and verify generated code is consistent.
- [x] 4.5 If any database schema changes are introduced, generate and validate Atlas migrations with the repository migration scripts.
