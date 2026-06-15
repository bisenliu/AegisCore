# Tasks

## 1. Inventory Current Input Flows

- [x] List every handler in `user-service/internal/features/*/transport/http/controller.go`.
- [x] Record each handler's current binding source: URI, query, JSON, header, or combined sources.
- [x] Record current post-bind helpers: `NormalizeXXX`, `ParseXXX`, and private parse helpers.
- [x] Record target application command/query for each handler.
- [x] Mark handlers with no request input, such as route diff and logout endpoints.

## 2. Lock Current Behavior

- [x] Review existing `validation_test.go` and controller tests for user, auth, role, and permission.
- [x] Add missing tests for current public error semantics before changing controllers.
- [x] Cover invalid UUID and cursor errors for user, role, and permission.
- [x] Cover trimmed empty auth credentials and token inputs.
- [x] Cover role binding request body parsing for single ID and ID list endpoints.

## 3. Add Input Preparers

- [x] Create `transport/http/input.go` for user.
- [x] Implement `prepareListUsersQuery`.
- [x] Implement `prepareCreateUserCommand`.
- [x] Implement `prepareGetUserByIDQuery`.
- [x] Create `transport/http/input.go` for auth.
- [x] Implement `prepareLoginCommand`.
- [x] Implement `prepareRefreshTokenCommand`.
- [x] Implement `prepareChangePasswordCommand`.
- [x] Create `transport/http/input.go` for permission.
- [x] Implement `prepareListPermissionsQuery`.
- [x] Implement `prepareCreatePermissionCommand`.
- [x] Implement `prepareGetPermissionQuery`.
- [x] Implement `prepareUpdatePermissionCommand`.
- [x] Implement `prepareSetPermissionActiveCommand`.
- [x] Implement `prepareUserEffectivePermissionsQuery`.
- [x] Create `transport/http/input.go` for role.
- [x] Implement `prepareListRolesQuery`.
- [x] Implement `prepareCreateRoleCommand`.
- [x] Implement `prepareUpdateRoleCommand`.
- [x] Implement `prepareSetRoleActiveCommand`.
- [x] Implement `prepareUserRolesQuery`.
- [x] Implement `prepareReplaceUserRolesCommand`.
- [x] Implement `prepareUserRoleCommand`.
- [x] Implement `prepareRolePermissionsQuery`.
- [x] Implement `prepareReplaceRolePermissionsCommand`.
- [x] Implement `prepareRolePermissionCommand`.

## 4. Add Multi-Source Binding Support

- [x] Decide whether to introduce generic `binding.Compose` immediately or start with feature-local private binders.
- [x] If generic support is chosen, add `Compose` to `common/http/binding`.
- [x] Add tests proving composed binders execute in order and return the first error.
- [x] Define combined request DTOs for URI + JSON endpoints.
- [x] Keep Swagger body DTOs stable where possible.

## 5. Refactor Controllers

- [x] Update user controller handlers to call one preparer after binding.
- [x] Update auth controller handlers to call one preparer after binding.
- [x] Update permission controller handlers to call one preparer after binding.
- [x] Update role controller handlers to call one preparer after binding.
- [x] Remove direct controller calls to `NormalizeXXX` and `ParseXXX`.
- [x] Keep business use case calls and response mapping unchanged.

## 6. Clean Up Old Helpers

- [x] After controller migration, search for direct helper usage with `rg "Normalize|Parse" user-service/internal/features/*/transport/http/controller.go`.
- [x] Keep old helper functions only if tests or non-controller code still use them intentionally.
- [x] Otherwise, inline them into preparers or make them private helpers near the preparer.
- [x] Ensure helper comments remain Chinese for maintained Go code.

## 7. Verify

- [x] Run `gofmt -w` on edited Go files.
- [x] Run `make test-user-service`.
- [x] Run `make test-common` if `common/http/binding` is changed.
- [x] Run `make swagger-generate` if request DTOs used in Swagger docs are changed.
- [x] Run `make lint-user-service` if the code change touches multiple feature controllers.

## 8. Documentation

- [x] Update `docs/ARCHITECTURE.md` HTTP request flow notes to mention feature-local input preparers.
- [x] Update `AGENTS.md` if the controller input convention becomes a repository rule.
- [x] Do not add `openspec/` or `docs/opsx/`.

## 9. Acceptance Checklist

- [x] Every modified controller handler has no more than one post-bind input-preparation call.
- [x] No controller directly calls `NormalizeXXX` or `ParseXXX`.
- [x] Multi-source endpoints bind into one request model.
- [x] Application command/query values are constructed in preparers.
- [x] Existing endpoint behavior and public errors remain compatible.
- [x] Tests pass for all touched modules.
