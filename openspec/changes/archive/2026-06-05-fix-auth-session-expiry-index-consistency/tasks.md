## 1. Redis Session Expiration Semantics

- [x] 1.1 Update `CreateSession` to normalize invalid `ttl`, compute a single `expiresAt` from `now + ttl`, and assign it to `session.ExpiresAt` before JSON serialization.
- [x] 1.2 Ensure `SET auth:session:<session_id>` TTL and `ZADD auth:user:<user_id>:sessions` score both use the same computed expiration source.
- [x] 1.3 Keep expired ZSet member cleanup in session creation, current-session deletion, and all-session deletion paths before reading or mutating active members.

## 2. User Session Index Cleanup Strategy

- [x] 2.1 Add a bounded expiration strategy for `auth:user:<user_id>:sessions` ZSet keys when creating sessions, without shortening an existing index TTL that protects longer-lived active sessions.
- [x] 2.2 Keep `DeleteAllUserSessions` behavior ordered as cleanup, read remaining members, delete session keys, then delete the user session index.
- [x] 2.3 Verify historical dirty members are handled safely: expired-score members are cleaned, missing session keys are harmless on `DEL`, and final all-session deletion removes the user index.

## 3. Consistency Boundary Evaluation

- [x] 3.1 Keep the initial implementation within the existing Redis repository boundary and `TxPipeline` transaction unless tests expose a consistency gap that requires Lua.
- [x] 3.2 If Lua is introduced, encapsulate it inside `user-services/internal/repository/redis` and cover script arguments, Redis time source, and error handling with tests.

## 4. Tests and Verification

- [x] 4.1 Add or update Redis auth session repository tests covering mismatch input where `session.ExpiresAt` differs from `ttl`; assert stored payload, session key TTL, and ZSet score align to the computed expiration.
- [x] 4.2 Add or update tests covering user session ZSet TTL or bounded cleanup behavior and ensuring shorter new sessions do not prematurely remove longer-lived index data.
- [x] 4.3 Add or update tests covering `DeleteAllUserSessions` with expired ZSet members and missing session keys.
- [x] 4.4 Run `gofmt` on changed Go files.
- [x] 4.5 Run `go test ./...` in `user-services/` and, if shared code changes are introduced, run `go test ./...` in `common/`.
