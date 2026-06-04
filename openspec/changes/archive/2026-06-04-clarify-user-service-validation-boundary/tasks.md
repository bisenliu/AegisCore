## 1. Validation Boundary Setup

- [x] 1.1 Inspect `user_controller.go`, `auth_controller.go`, DTO definitions, existing `common/validation` usage, and current controller/service tests to identify the exact request binding flow.
- [x] 1.2 Create `user-services/internal/validation` with focused functions for create-user input, get-user ID input, list-users query input, login input, change-password input, and refresh-token request normalization.
- [x] 1.3 Reuse existing `errmsg`, `common/response`, `common/validation`, and `response.NormalizePagination` behavior instead of adding new response or infrastructure helpers.

## 2. Controller Integration

- [x] 2.1 Update `UserController.Create` to call the user validation boundary after JSON binding and before `UserService.CreateUser`.
- [x] 2.2 Update `UserController.GetByID` to validate or parse `user_id` before calling `UserService.GetUserByID`.
- [x] 2.3 Update list-users controller flow to normalize pagination and filter fields before calling `UserService.ListUsers`.
- [x] 2.4 Update `AuthController.Login` to normalize and validate login credentials before calling `AuthService.Login`.
- [x] 2.5 Update `AuthController.ChangePassword` to normalize and validate the new password and request token boundary before calling `AuthService.ChangePassword`.
- [x] 2.6 Update `AuthController.Refresh` to normalize the request body refresh token, including optional Bearer prefix handling, before calling `AuthService.Refresh`.
- [x] 2.7 Keep public HTTP paths, request fields, response envelope, and existing error messages compatible.

## 3. Service Refactor

- [x] 3.1 Remove request-level `strings.TrimSpace` and empty-field checks from `CreateUser`, keeping username uniqueness, password hashing, ID generation, default status, logging, and error mapping in Service.
- [x] 3.2 Remove path-parameter UUID parsing from `GetUserByID`, or change the service input type to express an already validated UUID if the controller is the only caller.
- [x] 3.3 Remove list filter trimming and pagination normalization from `ListUsers`, ensuring Service uses normalized input from the validation boundary.
- [x] 3.4 Confirm Service still maps repository/domain errors to `common/response` application errors and never accesses HTTP/Gin request objects.

## 4. Auth Service Refactor

- [x] 4.1 Remove request-level username/password trimming and empty credential checks from `Login` and `authenticateUser`, while preserving invalid credential response semantics.
- [x] 4.2 Remove request-level new-password trimming and empty-password checks from `ChangePassword`, while preserving password-change token verification, user state checks, password hashing, credential update, and token-version invalidation.
- [x] 4.3 Remove request body refresh-token prefix stripping and empty-token checks from `Refresh`, while preserving JWT claims validation, subject validation, session lookup, token-version checks, rotation, and token issuance.
- [x] 4.4 Keep `verifyPasswordChangeToken`, refresh claims/session checks, and `authenticatedSession` security checks in Auth Service or auth middleware boundaries rather than ordinary request Validation.

## 5. Tests

- [x] 5.1 Add or update validation tests for trimmed create fields, blank-after-trim rejection, invalid UUID rejection, pagination normalization, list filter trimming, login credential trimming, blank login credential rejection, new-password trimming, blank new-password rejection, and refresh-token normalization.
- [x] 5.2 Add or update controller tests to verify HTTP 400 or authentication validation failures still use the expected response envelope and public messages.
- [x] 5.3 Add or update user service tests to focus on business behavior: uniqueness conflict, password hashing flow, user not found mapping, internal repository error mapping, and normalized list query inputs.
- [x] 5.4 Add or update auth service tests to focus on credential authentication, password-change token verification, refresh session/token-version behavior, logout context handling, and normalized auth inputs.
- [x] 5.5 Verify existing create/query/list/login/change-password/refresh/logout success behavior and response fields remain unchanged.

## 6. Verification

- [x] 6.1 Run `gofmt` on changed Go files.
- [x] 6.2 Run `go test ./...` in `user-services/`.
- [x] 6.3 Confirm no Ent schema, generated code, Atlas migration, route, config, Redis, or PostgreSQL runtime initialization changes were introduced.
