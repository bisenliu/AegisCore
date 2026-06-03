## 1. Repository Error Normalization

- [x] 1.1 Update `GetTokenVersion` in `user-services/internal/repository/postgres/user_repository.go` so Ent not found for missing or soft-deleted users returns `domain.ErrUserNotFound`.
- [x] 1.2 Update `IncrementTokenVersion` so missing or soft-deleted users return `domain.ErrUserNotFound` instead of `response.NotFoundError`.
- [x] 1.3 Update `UpdateCredentials` so missing or soft-deleted users return `domain.ErrUserNotFound` instead of `response.NotFoundError`.
- [x] 1.4 Remove no-longer-needed `common/response` or `user-services/internal/errmsg` imports from the PostgreSQL repository implementation if they become unused.

## 2. Service Mapping Verification

- [x] 2.1 Inspect auth/session service call sites for `GetTokenVersion`, `IncrementTokenVersion` and `UpdateCredentials` to confirm `domain.ErrUserNotFound` is mapped to the existing public failure semantics.
- [x] 2.2 Adjust service error mapping only if a call site would otherwise expose the new domain error as an internal error.

## 3. Tests

- [x] 3.1 Add or update repository tests proving token version lookup returns `domain.ErrUserNotFound` for a missing or soft-deleted user.
- [x] 3.2 Add or update repository tests proving token version increment returns `domain.ErrUserNotFound` for a missing or soft-deleted user.
- [x] 3.3 Add or update repository tests proving credential update returns `domain.ErrUserNotFound` for a missing or soft-deleted user and does not update soft-deleted users.
- [x] 3.4 Add or update service tests if needed to prove the public response semantics stay unchanged for these user-not-found paths.

## 4. Validation

- [x] 4.1 Run `gofmt` on changed Go files.
- [x] 4.2 Run `go test ./...` in `user-services/`.
- [x] 4.3 Confirm no Ent schema, generated code or Atlas migration changes were introduced.
