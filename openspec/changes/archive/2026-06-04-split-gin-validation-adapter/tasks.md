## 1. Core Split

- [x] 1.1 Add exported validation error classification API in `common/validation` for adapter response mapping.
- [x] 1.2 Create `common/ginvalidation` and move Gin binder implementations for JSON, strict JSON, URI, query and form binding.
- [x] 1.3 Move Gin-specific `Bind` and `BindOrAbort` behavior into `common/ginvalidation` while preserving logs, response envelopes and abort behavior.

## 2. Consumers

- [x] 2.1 Update user controllers to import `common/ginvalidation` for binder functions and `BindOrAbort`.
- [x] 2.2 Update validation and controller tests to use the new package boundaries.

## 3. Verification

- [x] 3.1 Run `gofmt` on modified Go files.
- [x] 3.2 Run `go test ./...` in `common/`.
- [x] 3.3 Run `go test ./...` in `user-services/`.
