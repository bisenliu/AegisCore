## 1. Repository Atomic Rotation

- [x] 1.1 Extend `repository.AuthSessionRepository` with an atomic Refresh Token rotation method that consumes an old session and creates a new session in one repository-level action.
- [x] 1.2 Implement the Redis rotation method in `user-services/internal/repository/redis/auth_session_repository.go` using Lua script, Redis transaction, or equivalent atomic mechanism.
- [x] 1.3 Map old-session-missing, old-session-mismatch, Redis command failure, and serialization failure paths to existing repository or response-compatible errors.
- [x] 1.4 Preserve existing Redis session key, user session ZSet key, TTL, expiration score, and stale member cleanup semantics during rotation.

## 2. Service Refactor

- [x] 2.1 Refactor `AuthService.Refresh` into a linear orchestration that validates input, parses and validates refresh session state, and selects rotation or non-rotation strategy.
- [x] 2.2 Add internal helpers or session lifecycle methods for `parseAndValidateRefreshSession`, `refreshWithoutRotation`, and `refreshWithRotation` or equivalent names with the same responsibilities.
- [x] 2.3 Move low-level new-session creation and old-session consumption for rotation behind `AuthSessionLifecycle` and repository abstraction boundaries.
- [x] 2.4 Ensure token signing failure still preserves the old refresh session and atomic rotation failure never returns the newly signed token.

## 3. Tests

- [x] 3.1 Add Redis repository tests for successful atomic rotation, missing old session, user or token version mismatch, TTL/index updates, and stale index cleanup.
- [x] 3.2 Add a concurrent rotation test proving multiple refresh attempts against the same old session produce at most one successful rotation.
- [x] 3.3 Update service tests for rotation success, rotation disabled behavior, token signing failure, and atomic rotation failure response semantics.
- [x] 3.4 Update existing stubs and component tests to satisfy the expanded session repository and lifecycle contracts.

## 4. Validation

- [x] 4.1 Run `gofmt -w` on modified Go files.
- [x] 4.2 Run `go test ./...` in `user-services/`.
- [x] 4.3 Run `go test ./...` in `common/` if shared interfaces or common code are touched.
- [x] 4.4 Run `openspec status --change "analyze-auth-refresh-concurrency"` and confirm the change remains apply-ready.
