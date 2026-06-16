# Tasks

## Implementation

- [x] Create `common/http/openapi/`.
- [x] Add `common/http/openapi/options.go` with `ConvertOptions`, `Server`, `SecurityScheme`, `Document` and `GoDocumentOptions`.
- [x] Add `common/http/openapi/convert.go` with Swagger 2 JSON to OpenAPI 3 conversion, validation and JSON/YAML serialization.
- [x] Add `common/http/openapi/render.go` with Go embed document rendering and raw string literal handling.
- [x] Move reusable logic from `user-service/internal/tools/openapi-convert/main.go` into the new shared package.
- [x] Ensure `common/http/openapi` does not hardcode `/api/v1`, health probe paths, `BearerAuth`, user-service descriptions or user-service script names.
- [x] Update `common/go.mod` so `github.com/getkin/kin-openapi` is a direct dependency.
- [x] Ensure `gopkg.in/yaml.v3` is a direct `common` dependency if used by the shared package.
- [x] Update `user-service/internal/tools/openapi-convert/main.go` to a thin wrapper around `common/http/openapi`.
- [x] Keep existing wrapper flags `-input`, `-json`, `-yaml`, `-go` and `-package`.
- [x] Add or retain wrapper-level user-service defaults for OpenAPI version, `/api/v1`, root health paths, `BearerAuth`, BearerAuth description and generated-by comment.
- [x] Update `user-service/scripts/openapi-generate.sh` if new wrapper flags need to be passed explicitly.
- [x] Run `gofmt` on modified Go files.
- [x] Run `go mod tidy` in `common/`.
- [x] Run `go mod tidy` in `user-service/` if direct OpenAPI dependencies changed.

## Tests

- [x] Add `common/http/openapi/convert_test.go` for Swagger 2 input conversion to OpenAPI 3.
- [x] Test global server injection through options.
- [x] Test path-level server injection through options.
- [x] Test security scheme injection through options.
- [x] Test empty optional normalization does not inject service-specific values.
- [x] Test invalid Swagger input returns a clear error.
- [x] Add `common/http/openapi/render_test.go` for Go embed rendering.
- [x] Test raw string literal rendering when JSON contains no backticks.
- [x] Test quoted string fallback when JSON contains backticks.
- [x] Add or update user-service wrapper tests only if wrapper behavior is not sufficiently covered by generation verification.

## Documentation

- [x] Update `AGENTS.md` to mention the shared OpenAPI conversion helper under `common/http/openapi` and clarify service-specific generation scripts remain service-owned.
- [x] Update `docs/ARCHITECTURE.md` Common Organization to include `common/http/openapi` as a build-time HTTP documentation helper.
- [x] Update `docs/ARCHITECTURE.md` user-service ownership text to keep OpenAPI generated docs and runtime OpenAPI routes in `user-service`.
- [x] Update `docs/DEVELOPMENT.md` OpenAPI generation guidance to describe the shared converter and service-owned `swag init` scan configuration.
- [x] Confirm no new `openspec/` or `docs/opsx/` artifacts were added.

## Verification

- [x] Run `cd common && go test ./http/openapi`.
- [x] Run `make openapi-generate`.
- [x] Run `cd user-service && go test ./internal/router`.
- [x] Run `make architecture-lint`.
- [x] Run `make test-common`.
- [x] Run `make test-user-service`.
- [x] Review generated `user-service/docs/openapi.json`, `user-service/docs/openapi.yaml` and `user-service/docs/openapi.go` diff for semantic equivalence.
- [x] Review `git diff` to confirm the change only touches the shared converter, user-service wrapper/script, related docs, module files and generated OpenAPI artifacts.

## Review Notes

- [x] Confirm `common/http/openapi` has no import dependency on `user-service`.
- [x] Confirm `common/http/openapi` does not import Gin, Ent, Redis, Fx, datastore providers, logger providers or feature packages.
- [x] Confirm service-specific OpenAPI semantics are passed through options or remain in the user-service wrapper.
- [x] Confirm future services can reuse the shared package without inheriting user-service paths or authentication descriptions.
