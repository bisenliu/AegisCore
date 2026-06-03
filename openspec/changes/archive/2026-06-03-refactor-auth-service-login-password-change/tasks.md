## 1. Auth Service Refactor

- [x] 1.1 Update `Login()` to trim request credentials, call a private `authenticateUser(ctx, username, plainPassword)` helper, and keep only authenticated-user token issuance branching in `Login()`.
- [x] 1.2 Implement `authenticateUser(ctx, username, plainPassword)` in `user-services/internal/service/auth_service.go` to handle empty credentials, username lookup, not-found mapping, shared password verification, password verification logging, and disabled-user rejection.
- [x] 1.3 Ensure `UserStatusMustChangePassword` remains a successful authentication outcome handled by `Login()` through `issuePasswordChangeToken()` rather than an authentication failure inside `authenticateUser()`.

## 2. Password Change Token Verification

- [x] 2.1 Update `ChangePassword()` to call a private `verifyPasswordChangeToken(ctx, token)` helper before user lookup and to continue handling user lookup, `status=300` validation, password hashing, credential update, and cache invalidation.
- [x] 2.2 Implement `verifyPasswordChangeToken(ctx, token) (uuid.UUID, error)` to normalize optional bearer prefix, parse password-change token, verify current token version, parse `user_id` as UUID, and return existing token invalid errors for invalid credentials.
- [x] 2.3 Keep password-change error handling and logging compatible with existing behavior, including not logging token contents or password material.

## 3. Verification

- [x] 3.1 Run `gofmt` on modified Go files.
- [x] 3.2 Run `go test ./...` in `user-services/` and fix any regressions.
- [x] 3.3 Run `go test ./...` in `common/` if shared auth or password code is touched.
- [x] 3.4 Verify OpenSpec status for `refactor-auth-service-login-password-change` remains apply-ready after implementation artifacts are complete.
