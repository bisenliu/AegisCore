## 1. App Boundary Updates

- [x] 1.1 Extend `authapp.AuthSessionStore` with a token version cache eviction method for revocation compensation.
- [x] 1.2 Extend `authapp.AuthSessionLifecycle` with a method that revokes Redis session projections at a supplied `token_version` without incrementing PostgreSQL.
- [x] 1.3 Update test fakes in `user-services/internal/features/auth/app` to implement the new narrow methods.

## 2. Revocation Flow Implementation

- [x] 2.1 Update `AuthService.ChangePassword` to use `CredentialUpdateResult.TokenVersion` and avoid calling the PostgreSQL-incrementing `RevokeAllUserSessions`.
- [x] 2.2 Update `authSessionLifecycle.RevokeAllUserSessions` so logout-all still increments PostgreSQL once, then performs Redis projection as best-effort compensation.
- [x] 2.3 Implement the supplied-version Redis projection flow: monotonic cache refresh, cache eviction on refresh failure, and idempotent refresh session deletion.
- [x] 2.4 Update `currentTokenVersion` so cache miss, invalid cache, and Redis read errors fall back to PostgreSQL; Redis backfill failure must log but not fail the current validation.
- [x] 2.5 Preserve existing not found, token invalid, and internal error mappings when PostgreSQL reads or increments fail.

## 3. Redis Store Implementation

- [x] 3.1 Implement monotonic `CacheTokenVersion` behavior in `features/auth/infra/redis/session_store.go` using Lua or an equivalent Redis atomic operation.
- [x] 3.2 Implement token version cache eviction for compensation without changing Redis key format.
- [x] 3.3 Keep refresh session creation, rotation, single-session deletion, all-session deletion, TTL fallback, and user session ZSet behavior compatible.

## 4. Tests

- [x] 4.1 Add app-level tests proving password change increments PostgreSQL `token_version` exactly once.
- [x] 4.2 Add app-level tests proving password change returns success after PostgreSQL credential update even when Redis projection fails, while recording/returning projection failure only at the compensation boundary.
- [x] 4.3 Add app-level tests proving logout-all commits PostgreSQL revocation and does not report a false business failure when Redis cache/session projection fails after the commit.
- [x] 4.4 Add token version resolver tests for cache miss, invalid cache, Redis read error fallback, PostgreSQL user-not-found, PostgreSQL unexpected error, and cache backfill failure.
- [x] 4.5 Add Redis store tests proving older cache writes do not overwrite newer token version values.
- [x] 4.6 Add Redis store tests proving cache eviction causes `GetCachedTokenVersion` to return cache miss and preserves existing key prefixes.
- [x] 4.7 Update existing tests that asserted cache refresh failure makes password change fail to reflect the new PostgreSQL-source-of-truth behavior.

## 5. Validation

- [x] 5.1 Run `gofmt -w` on modified Go files.
- [x] 5.2 Run `go test ./...` in `user-services/`.
- [x] 5.3 Run `go test ./...` in `common/` only if shared middleware or common auth contracts are modified.
- [x] 5.4 Verify no Ent schema, generated Ent code, Atlas migration, HTTP route, response envelope, JWT claim, config key, or Redis key format changes were introduced.
- [x] 5.5 Run `openspec status --change "fix-auth-revocation-consistency"` and confirm the change is ready for apply/archive workflow.
