## 1. PostgreSQL Lifecycle Ownership

- [x] 1.1 Review `common/runtime/datastorefx/postgres.go`, `user-services/internal/bootstrap/postgres.go`, and `user-services/internal/bootstrap/ent.go` to identify every place that opens, pings, closes, or rolls back PostgreSQL resources.
- [x] 1.2 Refactor the shared PostgreSQL helper or add a focused helper so each named `*sql.DB` pool has exactly one lifecycle owner for startup ping and stop close.
- [x] 1.3 Refactor user-service `ProvidePostgresPools` so `user_db` and `common_db` creation keeps opt-in behavior and handles second-pool creation failure with consistent rollback and named error context.
- [x] 1.4 Refactor Ent client lifecycle so `user_db` and `common_db` Ent clients do not duplicate ownership of the underlying shared `*sql.DB` pool close responsibility.
- [x] 1.5 Preserve existing Fx named injection names and runtime resource names for `user_db`, `common_db`, and `cache_redis`.

## 2. Tests

- [x] 2.1 Update common datastorefx tests to verify PostgreSQL helper still opens only the declared `postgres.<name>` instance, uses configured ping timeout, and registers the single expected pool lifecycle.
- [x] 2.2 Update user-service PostgreSQL pool tests to verify `ProvidePostgresPools` creates only `user_db` and `common_db`, does not create `pay_db`, and preserves named configuration behavior.
- [x] 2.3 Add or update a failure test where `common_db` creation fails after `user_db` creation, verifying `user_db` is rolled back and the returned error keeps `common_db` context.
- [x] 2.4 Add or update a composed lifecycle test for PostgreSQL pools plus Ent clients, verifying Ent cleanup does not establish a second close owner for the same underlying `*sql.DB` pools.
- [x] 2.5 Add or update stop-error tests to verify PostgreSQL pool close errors preserve named context and multiple close failures are not lost.

## 3. Validation

- [x] 3.1 Run `gofmt -w` on modified Go files.
- [x] 3.2 Run `go test ./...` in `common/`.
- [x] 3.3 Run `go test ./...` in `user-services/`.
- [x] 3.4 Confirm no Ent schema, generated Ent code, Atlas migration, HTTP API, response contract, Redis lifecycle, YAML key, or `AEGISCORE_` environment variable changes were introduced.
