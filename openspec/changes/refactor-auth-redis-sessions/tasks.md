## 1. Redis Key Contract

- [x] 1.1 Update `user-services/internal/features/auth/domain/rediskeys.go` so auth session keys use `auth:session:{<userID>}:<sessionID>`, user session indexes use `auth:user:sessions:{<userID>}`, and token version keys use `auth:user:token_version:{<userID>}`.
- [x] 1.2 Preserve `config.App.Name` auth Redis key prefix generation and update tests so the service-name prefix wraps the new hash-tagged key format.
- [x] 1.3 Update all `sessionStore` helper methods and call sites to pass `userID` when constructing a session payload key.

## 2. Lua-Based Session Operations

- [x] 2.1 Replace the WATCH/MULTI implementation of `RotateSession` in `user-services/internal/features/auth/infra/redis/session_store.go` with a Redis Lua script.
- [x] 2.2 Map Lua rotation return codes to `ErrAuthSessionNotFound`, `ErrAuthSessionMismatch`, or wrapped Redis errors without adding retry loops.
- [x] 2.3 Replace `DeleteAllUserSessions` with a Redis Lua script that cleans expired ZSet members, reads non-expired session IDs, and deletes payload keys plus the index using `UNLINK`.
- [x] 2.4 Keep `CreateSession`, `GetSession`, `DeleteSession`, token version cache operations, TTL derivation, and index TTL behavior aligned with the new key format and existing app/domain boundaries.

## 3. Tests

- [x] 3.1 Update `rediskeys_test.go` to assert the exact new key strings, including app-name-prefixed and empty-app-name variants.
- [x] 3.2 Update `session_store_test.go` key assertions for create/get/delete, token version cache, logout current, logout all, and app-name cases.
- [x] 3.3 Add or update tests proving old-format Redis keys are ignored for session lookup and token version cache lookup.
- [x] 3.4 Add or update tests for Lua rotation success, missing old session, old session mismatch, and concurrent attempts succeeding once.
- [x] 3.5 Add or update tests for Lua + `UNLINK` logout-all behavior, including expired index member cleanup and deletion of all non-expired session payload keys.

## 4. Verification

- [x] 4.1 Run `gofmt -w` on modified Go files.
- [x] 4.2 Run `go test ./...` in `user-services/`.
- [x] 4.3 Run `go test ./...` in `common/` if shared imports or contracts are touched.
- [x] 4.4 Confirm no Ent schema, generated Ent code, Atlas migration, HTTP DTO, route, response envelope, or JWT claim changes were introduced.
