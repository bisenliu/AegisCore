## 1. Repository Contract

- [x] 1.1 Add `UpdateCredentialsInput` to `user-services/internal/repository` with `UserID`, `PasswordHash`, and `Status` fields.
- [x] 1.2 Rename `UserRepository.UpdatePasswordHashAndStatus` to `UpdateCredentials` and update the concrete repository implementation.
- [x] 1.3 Preserve existing repository behavior: update only non-deleted users, set `password_hash`, set status, increment `token_version`, map zero updated rows to user not found, and return the refreshed `token_version`.

## 2. Auth Service Integration

- [x] 2.1 Update `authService.ChangePassword` to call `UpdateCredentials` with the new input structure.
- [x] 2.2 Keep password hashing, password-change token validation, status validation, Redis token version invalidation, and response behavior unchanged.

## 3. Tests And Stubs

- [x] 3.1 Update repository stub implementations in service and session store tests to satisfy the renamed interface.
- [x] 3.2 Update auth service tests to assert the new `UpdateCredentialsInput` values and preserve password hash, status, and token version expectations.
- [x] 3.3 Update any user service test stubs or compile-time references to remove `UpdatePasswordHashAndStatus`.

## 4. Verification

- [x] 4.1 Run `gofmt` on modified Go files.
- [x] 4.2 Run `go test ./...` in `user-services/`.
- [x] 4.3 Confirm no Ent schema, generated Ent files, Atlas migration, HTTP DTO, route, response code, or config changes were introduced.
