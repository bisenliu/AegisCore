## Why

`UserController.ListUsers` currently calls `validators.NormalizeListUsers(&req)` directly after query binding, which makes the HTTP controller responsible for pagination defaults, offset/limit derivation, and filter trimming. This leaks use-case input normalization into the transport layer and means non-HTTP callers of `UserService.ListUsers` can bypass the normalization that repository access expects.

## What Changes

- Move user list request normalization for pagination and filter trimming from `UserController.ListUsers` into `UserService.ListUsers` before repository access.
- Keep the controller responsible only for Gin query binding, shared validation, service invocation, and response output.
- Keep the existing `user-services/internal/validators.NormalizeListUsers` helper as the service-local normalization boundary, but invoke it from the service for list queries.
- Clarify the validation boundary for future request-specific rules: request DTO/shared validation handles structural checks, while service-level business validation and normalization that affects use-case execution happens in the service flow.
- Preserve external behavior: `GET /api/v1/users` path, query parameters, response envelope, pagination defaults, filter behavior, authentication requirements, and error semantics remain compatible.

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-profile-query`: tighten the list-query normalization requirement so pagination defaults, offset/limit derivation, and filter trimming are guaranteed inside the service flow before repository access, not in the HTTP controller.
- `request-validation`: clarify service-local validation boundaries so controller-level validation remains limited to binding and structural request validation, while business-state validation, side-effectful normalization, and checks requiring repositories/cache belong to service orchestration.

## Impact

- Affected code: `user-services/internal/controller/user_controller.go`, `user-services/internal/service/user_service.go`, and related controller/service tests.
- Affected specs: `openspec/specs/user-profile-query/spec.md` and `openspec/specs/request-validation/spec.md` via change deltas.
- API compatibility: no HTTP route, request field, response envelope, status code, business code, or pagination behavior changes.
- Data/model impact: no database schema, Ent schema, Atlas migration, Redis key, or configuration changes.
