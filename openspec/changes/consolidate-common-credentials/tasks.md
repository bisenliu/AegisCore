## 1. Create Credentials Package

- [x] 1.1 Create `common/credentials/password.go` with Argon2id password credential logic exposed as `HashPassword` and `VerifyPassword`, preserving existing hash format, default parameters, and error behavior.
- [x] 1.2 Create `common/credentials/jwt.go` with JWT credential service, claims, sign input, token subject constants, Bearer token type constant, and access/refresh/password-change token signing and parsing behavior.
- [x] 1.3 Create `common/credentials/context.go` with Authorization/Bearer constants, Gin/context key constants, and `WithUserID`/`UserIDFromContext`/`WithSessionID`/`SessionIDFromContext` helpers.
- [x] 1.4 Keep non-credential helpers such as trace-id, logger context, config loading, datastore wiring, response envelope, and middleware policy code outside `common/credentials`.

## 2. Migrate Callers

- [x] 2.1 Update `common/middleware/auth.go` to use `common/credentials` for JWT service types, Authorization/Bearer constants, and user/session context propagation.
- [x] 2.2 Update `common/middleware/cors.go` and common tests to read Authorization header constants from `common/credentials`.
- [x] 2.3 Update `user-services/internal/bootstrap` JWT provider wiring to construct credentials JWT service from existing auth config.
- [x] 2.4 Update user-services auth, user creation, controller, and tests to use `common/credentials` instead of `common/jwt`, `common/password`, or auth-specific `common/contextutil` APIs.
- [x] 2.5 Search the workspace for remaining `common/jwt`, `common/password`, `common/contextutil` auth usages and remove or justify any remaining references.

## 3. Package Cleanup

- [x] 3.1 Remove migrated `common/password` implementation after all internal callers use `common/credentials`, unless a temporary wrapper is needed for compilation during migration.
- [x] 3.2 Remove migrated `common/jwt` implementation after all internal callers use `common/credentials`, unless a temporary wrapper is needed for compilation during migration.
- [x] 3.3 Remove auth-specific contents from `common/contextutil/auth.go` if no non-auth context utilities remain in that file.
- [x] 3.4 Ensure no deleted package leaves stale tests, imports, or package directories that break `go test ./...`.

## 4. Tests And Verification

- [x] 4.1 Add or move credentials password tests covering successful hash/verify, empty password, invalid hash, and mismatched password.
- [x] 4.2 Add or move credentials JWT tests covering signing/parsing access, refresh, and password-change tokens; missing secret; invalid subject; missing identity fields; expired token; issuer/audience behavior.
- [x] 4.3 Add or move credentials context tests covering user ID/session ID write/read and nil or missing context behavior.
- [x] 4.4 Run `gofmt` on changed Go files.
- [x] 4.5 Run `go test ./...` in `common/`.
- [x] 4.6 Run `go test ./...` in `user-services/`.

## 5. Documentation And Specs Alignment

- [x] 5.1 Update `docs/opsx/CAPABILITY_MAP.md` to include `common-credentials` and adjust `user-authentication` code locations from split credential packages to `common/credentials`.
- [x] 5.2 Verify the implemented behavior satisfies `openspec/changes/consolidate-common-credentials/specs/common-credentials/spec.md`.
- [x] 5.3 Verify the implemented behavior satisfies the `user-authentication` delta spec and does not change HTTP 401 response envelope, token values, or existing auth config behavior.
