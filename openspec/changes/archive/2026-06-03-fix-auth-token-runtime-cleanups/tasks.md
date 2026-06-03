## 1. Shared Common Helpers

- [x] 1.1 Update `common/infrastructure/logger.go` so logger stop sync ignores both `syscall.EINVAL` and `syscall.ENOTTY` while preserving all other sync errors.
- [x] 1.2 Add `common/auth.StripBearerPrefix(token string) string` with trim, case-insensitive `Bearer ` prefix matching, and raw-token fallback behavior.
- [x] 1.3 Add or update common auth tests for raw tokens, whitespace trimming, canonical `Bearer `, and mixed-case bearer prefix inputs.
- [x] 1.4 Replace the bare encoded hash length literal in `common/password/password.go` with a `maxEncodedHashLength = 512` constant.

## 2. User Service Token Cleanup

- [x] 2.1 Replace `normalizeRefreshToken()` usage in `user-services/internal/service/auth_service.go` with `auth.StripBearerPrefix()` and delete the local helper.
- [x] 2.2 Ensure password-change token verification in `auth_service.go` also uses `auth.StripBearerPrefix()` and keeps empty-token handling unchanged.
- [x] 2.3 Replace `bearerToken()` usage in `user-services/internal/controller/auth_controller.go` with `auth.StripBearerPrefix()` or an import alias, then delete the local helper.
- [x] 2.4 Keep `authenticatedSession()` private in service code for this change and avoid adding response or errmsg dependencies to `common/auth`.

## 3. Runtime Constant Cleanup

- [x] 3.1 Add `defaultShutdownTimeout = 10 * time.Second` in `user-services/internal/bootstrap/bootstrap.go` and use it in the HTTP server stop hook fallback.
- [x] 3.2 Add separate `startTimeout = 15 * time.Second` and `stopTimeout = 15 * time.Second` constants in `user-services/cmd/main.go` and use them for Fx app start and stop contexts.

## 4. Verification

- [x] 4.1 Run `gofmt` on modified Go files.
- [x] 4.2 Run common module tests with `go test ./...` from `common/`.
- [x] 4.3 Run user-services module tests with `go test ./...` from `user-services/`.
- [x] 4.4 Verify OpenSpec change status reports all apply-required artifacts complete.
