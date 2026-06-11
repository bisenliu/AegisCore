## 1. Error Classification Contract

- [x] 1.1 Define a common-layer token version mismatch classification path for `common/http/middleware.AuthWithTokenVersionValidator` without importing `user-services` packages from `common`.
- [x] 1.2 Ensure the injected `authapp.TokenVersionValidator` can expose `authdomain.ErrTokenVersionMismatch` to the common-layer classifier through wrapping or adapter logic.
- [x] 1.3 Verify non-mismatch errors from the validator remain ordinary infrastructure errors and are not converted to `ErrTokenInvalid`.

## 2. Auth App Token Version Resolver

- [x] 2.1 Update `user-services/internal/features/auth/app/sessions.go` so Redis cache read errors other than `ErrTokenVersionCacheMiss` return an infrastructure error instead of only warning and falling back silently.
- [x] 2.2 Update token version cache miss fallback so PostgreSQL read failures are returned as infrastructure errors with preserved context.
- [x] 2.3 Update Redis cache backfill failure after successful PostgreSQL fallback to return an infrastructure error for the middleware validation path.
- [x] 2.4 Keep explicit token version mismatch returning `authdomain.ErrTokenVersionMismatch` and keep malformed `user_id` claims rejected before repository access.

## 3. Middleware Response Mapping

- [x] 3.1 Update `common/http/middleware/auth.go` so token version mismatch returns HTTP 401 with token invalid response semantics.
- [x] 3.2 Update `common/http/middleware/auth.go` so Redis, PostgreSQL, cache backfill, or other unexpected validator errors return the internal error envelope with HTTP 500 and code `90000`.
- [x] 3.3 Ensure infrastructure-error paths abort the request and never call the protected route handler.
- [x] 3.4 Ensure infrastructure-error logs are error level, preserve trace-aware context and `user_id`, and do not log raw JWTs or sensitive claims.

## 4. Tests

- [x] 4.1 Add or update common middleware tests for token version mismatch returning token invalid instead of internal error.
- [x] 4.2 Add common middleware tests for validator infrastructure errors returning HTTP 500/code `90000` and not executing the handler.
- [x] 4.3 Add auth app tests for Redis cache read failure, PostgreSQL fallback failure, and Redis cache backfill failure returning infrastructure errors.
- [x] 4.4 Add or update auth app tests confirming current-version mismatch still returns `authdomain.ErrTokenVersionMismatch`.
- [x] 4.5 Update existing route/bootstrap tests that previously expected generic validator errors to map to token invalid.

## 5. Verification

- [x] 5.1 Run `gofmt` on modified Go files.
- [x] 5.2 Run `go test ./...` in `common/`.
- [x] 5.3 Run `go test ./...` in `user-services/`.
- [x] 5.4 Confirm no Ent schema, generated Ent code, Atlas migration, Redis key format, config key, or HTTP route changes were introduced.
