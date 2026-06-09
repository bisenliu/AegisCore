## 1. Password Package Implementation

- [x] 1.1 Update `common/security/password/password.go` to delete `Hash` and `Verify`, and expose only `HashContext` and `VerifyContext` for password hash and verify operations.
- [x] 1.2 Add package-level KDF queue and concurrency gate using buffered channels, returning `ErrPasswordKDFBusy` when the total in-flight/waiting limit is reached.
- [x] 1.3 Add `ErrPasswordTooLong`, `maxPasswordLength`, plain password validation, and shared validation use from hash and verify paths.
- [x] 1.4 Tighten encoded hash parsing to reject unsupported Argon2id parameters, salt length, and key length before deriving keys.
- [x] 1.5 Migrate all repository call sites from `password.Hash` and `password.Verify` to `password.HashContext` and `password.VerifyContext` using existing request or service context.
- [x] 1.6 Decide during implementation whether `password.go` remains maintainable as a single file; split only if the final file becomes materially harder to navigate.

## 2. Tests

- [x] 2.1 Update `common/security/password/password_test.go` to verify `HashContext` and `VerifyContext` preserve existing hash format and matching behavior without using deleted `Hash`/`Verify` APIs.
- [x] 2.2 Add tests for empty password compatibility, oversized password rejection, malformed hash rejection, unsupported parameter rejection, and salt/key length policy rejection.
- [x] 2.3 Add tests for context cancellation while waiting for KDF capacity and `ErrPasswordKDFBusy` when the queue limit is reached.
- [x] 2.4 Ensure tests avoid expensive or flaky timing assumptions by controlling package-level gate/queue state within the password package test boundary.

## 3. Verification

- [x] 3.1 Run `gofmt -w` on modified Go files.
- [x] 3.2 Run `go test ./...` from `common/`.
- [x] 3.3 Run relevant `user-services` tests or targeted package tests if call sites were migrated outside `common`.
- [x] 3.4 Confirm no `user-services/ent/`, Atlas migration, runtime config, Redis, PostgreSQL, HTTP route, or response contract changes were introduced.
- [x] 3.5 Confirm no repository code still references `password.Hash` or `password.Verify`.
