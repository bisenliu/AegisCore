## 1. Service Boundary

- [x] 1.1 Add `UserProfileStore`, `CreateUserInput`, and `ListUsersInput` to `user-services/internal/service`.
- [x] 1.2 Update `NewUserService` and `userService` to depend on `UserProfileStore` instead of repository package interfaces.
- [x] 1.3 Replace service-layer uses of `repository.CreateUserInput` and `repository.ListUsersInput` with service-owned input models.

## 2. Repository Adapter

- [x] 2.1 Update PostgreSQL user repository method signatures to implement `service.UserProfileStore`.
- [x] 2.2 Remove the root `repository.UserProfileRepository` interface and profile input models from the repository package if no longer consumed.
- [x] 2.3 Remove `postgres.AsUserProfileRepository` and any compile-time assertions that reference the removed repository profile interface.

## 3. Dependency Injection

- [x] 3.1 Update Fx providers in bootstrap to bind the PostgreSQL implementation to `service.UserProfileStore`.
- [x] 3.2 Preserve existing bindings for authentication credential and token version repository interfaces.
- [x] 3.3 Verify application bootstrap still resolves controller, service, repository, Redis, PostgreSQL, and Ent dependencies in tests.

## 4. Tests And Validation

- [x] 4.1 Update user service tests and fakes to use service-owned input models and profile store interface.
- [x] 4.2 Run `gofmt` on modified Go files.
- [x] 4.3 Run `go test ./...` in `user-services`.
- [x] 4.4 Run `go test ./...` in `common` if shared package imports or workspace-level assumptions are affected.
