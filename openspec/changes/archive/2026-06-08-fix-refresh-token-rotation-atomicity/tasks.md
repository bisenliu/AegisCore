## 1. Current Flow Verification

- [x] 1.1 Review `user-services/internal/service/auth_service.go` refresh orchestration and identify the exact delete/sign/create ordering under `refreshTokenRotation`.
- [x] 1.2 Review `AuthTokenIssuer`, `AuthSessionManager`, and `repository.AuthSessionRepository` APIs to determine whether the minimal service-level reorder is sufficient or a repository-level atomic rotation API is required.
- [x] 1.3 Confirm existing tests around refresh rotation, session creation, session deletion, and token issuance failure paths.

## 2. Core Implementation

- [x] 2.1 Update Refresh Token rotation so new token signing failure does not delete the validated old session.
- [x] 2.2 Update Refresh Token rotation so new Redis session creation failure does not delete the validated old session and does not return an unusable Refresh Token.
- [x] 2.3 Handle old session deletion failure after new session creation by returning failure without exposing the new Refresh Token.
- [x] 2.4 Add best-effort cleanup for a newly created session when old session revocation fails, with trace-aware warning logs if cleanup fails.
- [x] 2.5 If strict replay prevention is selected, add a repository/session-manager atomic rotation operation backed by Redis transaction or Lua script and keep service orchestration at the business boundary.

## 3. Tests

- [x] 3.1 Add unit tests proving token signing failure leaves the old refresh session valid when rotation is enabled.
- [x] 3.2 Add unit tests proving new session creation failure leaves the old refresh session valid and does not return a new token response.
- [x] 3.3 Add unit tests proving old session deletion failure does not return the newly issued token and attempts cleanup of the newly created session.
- [x] 3.4 Add success-path tests proving rotation returns a new Refresh Token only after the new session exists and the old session is revoked.
- [x] 3.5 If atomic Redis rotation is implemented, add repository tests for success, old session missing, concurrent reuse, and script/transaction failure paths.

## 4. Validation

- [x] 4.1 Run `gofmt -w` on modified Go files.
- [x] 4.2 Run `go test ./...` in `user-services/`.
- [x] 4.3 Run `go test ./...` in `common/` if shared auth/session contracts or common code changed.
- [x] 4.4 Verify no Ent schema, generated Ent code, Atlas migration, HTTP route, response envelope, JWT claim, or Redis key format changes were introduced.
