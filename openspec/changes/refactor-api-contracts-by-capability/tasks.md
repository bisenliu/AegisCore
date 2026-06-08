## 1. API Contract Package Structure

- [x] 1.1 Create `user-services/internal/api/user` with `request.go`, `response.go`, and `doc.go` using package name `userapi`.
- [x] 1.2 Move `GetUserRequest`, `ListUsersRequest`, `CreateUserRequest`, and `CreateUserRequest.SetDefaults` into the user API request file without changing fields, tags, defaults, or comments semantics.
- [x] 1.3 Move `UserResponse` into the user API response file without changing JSON fields, examples, or public response semantics.
- [x] 1.4 Move `UserListResponseDoc` into the user API doc file without changing pagination documentation structure.
- [x] 1.5 Create `user-services/internal/api/auth` with `request.go` and `response.go` using package name `authapi`.
- [x] 1.6 Move `LoginRequest`, `RefreshTokenRequest`, and `ChangePasswordRequest` into the auth API request file without changing fields, tags, or token input semantics.
- [x] 1.7 Move `TokenResponse`, `LogoutResponse`, and `ChangePasswordResponse` into the auth API response file without changing JSON fields or response semantics.

## 2. Reference Migration

- [x] 2.1 Update user controller imports, request binding, Swagger annotations, and response references from `dto.*` to `userapi.*`.
- [x] 2.2 Update auth controller imports, request binding, Swagger annotations, and response references from `dto.*` to `authapi.*`.
- [x] 2.3 Update user service interface, implementation, mapper return types, and pagination response generics to use `userapi.*`.
- [x] 2.4 Update auth service interface, implementation, token issuer component, and auth component tests to use `authapi.*`.
- [x] 2.5 Update service-side validation functions to accept `userapi.*` and `authapi.*` request types while preserving normalization behavior.
- [x] 2.6 Update all affected tests and fakes to import the new API contract packages.

## 3. Cleanup And Compatibility Checks

- [x] 3.1 Remove the obsolete `user-services/internal/dto` package after all references are migrated.
- [x] 3.2 Search the user service module for remaining `internal/dto` imports and `dto.` references and eliminate them.
- [x] 3.3 Verify no repository, domain, bootstrap runtime, Ent schema, Atlas migration, Redis key, configuration, or common module changes were introduced.
- [x] 3.4 Verify Swagger annotations no longer reference `dto.*` and still describe the same request and response fields.

## 4. Validation

- [x] 4.1 Run `gofmt` on changed Go files.
- [x] 4.2 Run `go test ./...` in `user-services` and fix any compile or behavior regressions.
- [x] 4.3 Run Swagger-related tests or generation checks available in `user-services` to confirm annotations compile against the new API contract packages.
- [x] 4.4 Confirm user create, user query/list, login, refresh, change-password, logout, and logout-all tests still cover unchanged HTTP contracts, request validation, success responses, and failure responses.
