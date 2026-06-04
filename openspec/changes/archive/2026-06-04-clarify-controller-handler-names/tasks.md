## 1. Controller Handler Renames

- [x] 1.1 Rename `UserController.List` to `UserController.ListUsers` and update its godoc comment anchor.
- [x] 1.2 Rename `UserController.Create` to `UserController.CreateUser` and update its godoc comment anchor.
- [x] 1.3 Rename `AuthController.Login` to `AuthController.LoginUser` and update its godoc comment anchor.
- [x] 1.4 Rename `AuthController.Refresh` to `AuthController.RefreshToken` and update its godoc comment anchor.
- [x] 1.5 Rename `AuthController.Logout` to `AuthController.LogoutCurrentSession` and update its godoc comment anchor.
- [x] 1.6 Rename `AuthController.LogoutAll` to `AuthController.LogoutAllSessions` and update its godoc comment anchor.

## 2. Route and Test Updates

- [x] 2.1 Update `user-services/internal/router/users.go` to register `ListUsers` and `CreateUser` without changing route paths or HTTP methods.
- [x] 2.2 Update `user-services/internal/router/auth.go` to register `LoginUser`, `RefreshToken`, `LogoutCurrentSession`, and `LogoutAllSessions` without changing route paths or HTTP methods.
- [x] 2.3 Update controller, router, bootstrap, or Swagger tests that refer to old handler names.
- [x] 2.4 Search for old controller handler references and remove stale direct references to `List`, `Create`, `Login`, `Refresh`, `Logout`, and `LogoutAll` where they refer to controller methods.

## 3. Verification

- [x] 3.1 Run `gofmt` on modified Go files.
- [x] 3.2 Run `go test ./...` in `user-services`.
- [x] 3.3 If common files are unexpectedly touched, run `go test ./...` in `common`; otherwise confirm no common code changed.
- [x] 3.4 Verify the OpenSpec delta with `openspec validate clarify-controller-handler-names --strict` or the repository's equivalent OpenSpec validation command.
