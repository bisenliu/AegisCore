# Tasks

## Implementation

- [x] Create `common/http/binding/`.
- [x] Move `common/http/ginvalidation/binder.go` to `common/http/binding/binder.go`.
- [x] Move `common/http/ginvalidation/validator.go` to `common/http/binding/validator.go`.
- [x] Move `common/http/ginvalidation/validation_test.go` to `common/http/binding/validation_test.go`.
- [x] Change moved files from `package ginvalidation` to `package binding`.
- [x] Update `user-service/internal/features/user/transport/http/controller.go` import path to `github.com/aegiscore/common/http/binding`.
- [x] Update user controller call sites from `ginvalidation.*` to `binding.*`.
- [x] Update `user-service/internal/features/auth/transport/http/controller.go` import path to `github.com/aegiscore/common/http/binding`.
- [x] Update auth controller call sites from `ginvalidation.*` to `binding.*`.
- [x] Remove the old `common/http/ginvalidation/` directory after files are migrated.
- [x] Run `gofmt` on modified Go files.

## Documentation

- [x] Update `docs/ARCHITECTURE.md` so `common/http/` references `common/http/binding` for Gin request binding and validation failure response adaptation.
- [x] Update `AGENTS.md` if any current repository rules refer to `ginvalidation`.
- [x] Update `docs/DEVELOPMENT.md` or `docs/TESTING.md` if they contain current guidance for the old package path.
- [x] Leave historical `docs/changes/*` old-path mentions unchanged unless they are being presented as current rules.

## Verification

- [x] Run `rg -n "ginvalidation" common user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md` and confirm there are no current code or long-term-rule references.
- [x] Run `cd common && go test ./http/binding ./validation`.
- [x] Run `cd user-service && go test ./internal/features/user/transport/http ./internal/features/auth/transport/http ./internal/bootstrap`.
- [x] If time allows, run `make test-common`.
- [x] If time allows, run `make test-user-service`.
- [x] Review `git diff` to confirm only package naming, import paths, docs and tests changed.

## Review Notes

- [x] Confirm no behavior changed in binding, validation, error classification or HTTP response output.
- [x] Confirm no `common/validation` core logic changed.
- [x] Confirm no compatibility shim keeps the old `ginvalidation` package alive.
- [x] Confirm no `openspec/` or `docs/opsx/` artifacts were added.
