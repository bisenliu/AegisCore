## 1. Shared Validation Package

- [x] 1.1 Create `common/validation` package with `Validator`, `Options`, `Binder`, `Defaultable`, `Validatable`, `Enum`, `Error`, and `FieldError` types.
- [x] 1.2 Implement validator initialization with `validator.New(validator.WithRequiredStructEnabled())`, Chinese translations, tag name resolution, and safe `enum` rule registration.
- [x] 1.3 Implement URI, query, JSON, and form binders without relying on controller-local validator construction.
- [x] 1.4 Implement normalized validation error handling for validator errors, JSON type mismatch, empty JSON body, and custom validation errors.
- [x] 1.5 Implement `Bind`, `Validate`, and `BindOrAbort` helpers that map request validation failures to `common/response.Envelope` HTTP 400 responses.
- [x] 1.6 Add `validation.Module` Fx provider without creating Redis, PostgreSQL, Ent, or HTTP server runtime dependencies.

## 2. Integration

- [x] 2.1 Add validation dependencies as direct dependencies in `common/go.mod` and run `go mod tidy` in `common`.
- [x] 2.2 Wire `validation.Module` into the user service Fx graph where controllers are constructed.
- [x] 2.3 Update `UserController` to inject `*validation.Validator` instead of constructing `validator.New(...)` locally.
- [x] 2.4 Migrate user ID URI validation to the shared validation helper while preserving `invalid user id` for non-numeric and non-positive IDs.

## 3. Tests

- [x] 3.1 Add `common/validation` unit tests for successful struct validation, required/gt failures, label/json/form/uri/query field names, and omitted fields.
- [x] 3.2 Add JSON binding tests for valid body, empty body, and type mismatch error normalization.
- [x] 3.3 Add extension hook tests for `SetDefaults()` before validation and `Validate() error` after struct validation.
- [x] 3.4 Add enum tests covering valid enum, invalid enum, nil pointer enum, and misconfigured non-enum fields without panic.
- [x] 3.5 Add or update user controller tests for `GET /api/v1/users/:id`, including valid ID, non-numeric ID, non-positive ID, not found, and service error paths.
- [x] 3.6 Add Fx validation test proving the user service graph can resolve the shared validation dependency.

## 4. Verification

- [x] 4.1 Run `gofmt -w` on changed Go files.
- [x] 4.2 Run `go test ./...` in `common/`.
- [x] 4.3 Run `go test ./...` in `user-services/`.
- [x] 4.4 Confirm no Ent generated files, schema files, Atlas migrations, Redis setup, or PostgreSQL setup were changed for this request-validation work.
