## 1. Schema And Domain Model

- [x] 1.1 Inspect current user schema, generated Ent model, DTOs, repository predicates, Swagger annotations and tests for all `active`、`name`、`password` references.
- [x] 1.2 Define a user-domain `status` enum with values `100` normal, `200` frozen/disabled and `300` must-change-password, including `IsValid() bool` for shared enum validation.
- [x] 1.3 Update `user-services/ent/schema/user.go` to replace `active` with non-null `status` default `100`, rename `name` to `nickname`, rename persistent `password` to `password_hash`, add nullable `deleted_at`, and update comments/index declarations.
- [x] 1.4 Decide and document the email uniqueness strategy with soft delete: keep full-table unique email or use a PostgreSQL `deleted_at IS NULL` partial unique index if supported by generated/reviewed migration SQL.

## 2. API DTOs And Validation

- [x] 2.1 Update create-user request DTOs and response DTOs to use `nickname` and `status`, keep external password input as `password`, and exclude `password_hash` and `deleted_at` from user responses.
- [x] 2.2 Update list-user query DTOs to use `nickname` and `status` filters instead of `name` and `active`.
- [x] 2.3 Add `validate:"enum"` to every request DTO field that accepts `status`, using the enum type from task 1.2 so validation flows through `common/validation` `validateEnum`.
- [x] 2.4 Update defaulting logic so omitted create-user `status` becomes `100` and optional query `status` can be omitted without failing enum validation.
- [x] 2.5 Remove controller/service hardcoded status value validation if introduced during implementation; status request validation must be centralized through the shared enum rule.

## 3. Repository And Service Behavior

- [x] 3.1 Update user creation flow to normalize email, accept `nickname`, hash or receive the service-produced password hash, persist `password_hash`, set `status`, and leave `deleted_at` null.
- [x] 3.2 Update query-by-ID repository methods to select only users where `deleted_at IS NULL` and map responses to `nickname` and `status`.
- [x] 3.3 Update list repository methods and count queries to filter only `deleted_at IS NULL`, support `nickname` contains filtering, normalized email filtering, and enum `status` filtering.
- [x] 3.4 Update user existence and uniqueness checks to use the selected soft-delete email uniqueness strategy from task 1.4.
- [x] 3.5 Update authentication/session repository and service code to read `password_hash`, require `deleted_at IS NULL`, reject `status=200` and `status=300` for normal login, and keep token version reads unavailable for soft-deleted users.
- [x] 3.6 Remove remaining runtime references to old persistent/user contract fields `active`, `name` and `password` except where an external request password input is intentionally named `password`.

## 4. Generated Code And Migrations

- [x] 4.1 Run `go generate ./ent` in `user-services` after Ent schema changes and verify generated code reflects `nickname`, `password_hash`, `status` and `deleted_at`.
- [x] 4.2 Run the user-service migration diff script with a semantic name such as `update_user_status_soft_delete_fields`.
- [x] 4.3 Review generated SQL to ensure existing `active` values migrate to `status`, existing `name` values migrate to `nickname`, existing `password` values migrate to `password_hash`, existing records get `deleted_at=NULL`, and old columns are removed only after data is preserved.
- [x] 4.4 Review and adjust generated indexes for `email`, `nickname`, `status` and `deleted_at IS NULL`; if SQL is manually edited, recalculate the Atlas migration checksum.
- [x] 4.5 Run the migration validation script and ensure `user-services/migrations/atlas.sum` is synchronized with the SQL files.

## 5. Swagger And Documentation Artifacts

- [x] 5.1 Update Swagger annotations for create, query, list and authentication-related schemas so docs use `nickname`, `status` and password input semantics correctly.
- [x] 5.2 Regenerate `user-services/docs/docs.go`, `user-services/docs/swagger.json` and `user-services/docs/swagger.yaml`.
- [x] 5.3 Verify generated Swagger does not expose `name`, `active`, `password_hash` or `deleted_at` as user response fields and documents status values `100`, `200`, `300`.

## 6. Tests

- [x] 6.1 Update existing create/query/list tests for response field names, default `status=100`, and absence of `password_hash`, `active`, `name` and `deleted_at`.
- [x] 6.2 Add validation tests for valid and invalid `status` values in JSON body and query parameters, asserting the shared validation failure envelope.
- [x] 6.3 Add repository/service tests confirming soft-deleted users are excluded from query-by-ID, list, count, login and token-version lookup paths.
- [x] 6.4 Add authentication tests confirming `password_hash` is used for credential verification and `status=200`/`status=300` users cannot create normal sessions.
- [x] 6.5 Add or update migration-related tests or validation fixtures covering field rename/data migration expectations where the repository already supports them.

## 7. Verification

- [x] 7.1 Run `gofmt` on changed Go files.
- [x] 7.2 Run `go test ./...` in `common/`.
- [x] 7.3 Run `go test ./...` in `user-services/`.
- [x] 7.4 Run the user-service migration validation script after any SQL or checksum edits.
- [x] 7.5 Inspect generated Swagger output and OpenSpec task/spec coverage before marking the change complete.

## 8. Must-Change-Password Restricted Authentication

- [x] 8.1 Extend JWT/auth token semantics to represent a password-change-only credential, for example with a dedicated subject or scope distinct from normal access and refresh tokens.
- [x] 8.2 Update login so `status=300` verifies `password_hash`, does not create a normal Redis session, and returns the restricted password-change credential instead of a generic credential error.
- [x] 8.3 Update auth middleware or password-change route guard so the restricted credential is accepted only by the modify-password endpoint and rejected by ordinary protected APIs.
- [x] 8.4 Update password-change handling so successful password change sets `status=100` and invalidates the restricted credential or makes it unusable for repeated password changes.
- [x] 8.5 Add tests for `status=300` login, ordinary API rejection with the restricted credential, password-change endpoint acceptance, and status transition back to `100`.
- [x] 8.6 Regenerate Swagger docs if login or password-change response DTOs change, then run `gofmt`, `go test ./...` in `common/` and `user-services/`.
