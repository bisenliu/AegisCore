## 1. Repository Interface Boundary

- [x] 1.1 Update `repository.AuthSessionRepository` so token version methods describe Redis cache operations instead of current-version DB fallback semantics.
- [x] 1.2 Refactor `repository/redis/auth_session_repository.go` to remove `UserTokenVersionRepository` from Fx params and struct fields.
- [x] 1.3 Keep Redis session record, user session index, token version cache key format, TTL fallback and invalid cache handling behavior compatible.

## 2. Service Composition

- [x] 2.1 Move cache hit, cache miss, PostgreSQL fallback and Redis cache backfill orchestration into `authSessionManager` or a private token version resolver.
- [x] 2.2 Ensure password-change token validation and Refresh Token validation use the new service-side token version resolver.
- [x] 2.3 Ensure logout-all and password-change revocation still increment PostgreSQL `token_version`, then invalidate Redis token version cache and Redis sessions.

## 3. Tests

- [x] 3.1 Update Redis auth session repository tests to verify cache-only behavior, Redis key compatibility, TTL handling, invalid cache reporting and absence of DB fallback dependency.
- [x] 3.2 Add or update service tests for token version cache hit, cache miss DB fallback, invalid cache fallback, DB user-not-found mapping and Redis read/write error propagation.
- [x] 3.3 Update test stubs affected by `AuthSessionRepository` interface changes.

## 4. Verification

- [x] 4.1 Run `gofmt` on changed Go files.
- [x] 4.2 Run `go test ./...` in `user-services/`.
- [x] 4.3 Run `go test ./...` in `common/` if shared code is touched.
- [x] 4.4 Confirm no Ent schema, generated Ent code or Atlas migration changes are required.
