# Controller Input Preparers

## What

Introduce a consistent HTTP controller input-preparation pattern for user-service features.

Each controller handler should keep request input handling to at most two visible steps:

1. `binding.BindOrAbort(...)` for binding and structural validation.
2. A single feature-local preparer call, such as `prepareListUsersQuery(req)` or `prepareReplaceUserRolesCommand(req)`, that combines current `NormalizeXXX`, `ParseXXX`, and DTO-to-command/query mapping work.

The intended controller shape is:

```go
req := SomeRequest{}
if !binding.BindOrAbort(ctl.validator, c, &req, binder) {
	return
}

input, err := prepareSomeInput(req)
if err != nil {
	response.Fail(c, err)
	return
}
```

Business invocation, business error mapping, response DTO mapping, and response output remain unchanged.

## Why

Current feature controllers already follow a good first step with `binding.BindOrAbort`, but the second-stage input work is split across many explicit controller calls:

- `NormalizeListUsers` then `ParseListCursor`
- `NormalizeCreateUser`
- `NormalizeLogin`, `NormalizeRefresh`, `NormalizeChangePassword`
- `ParseRoleID`, `ParseUserRoleIDs`, `ParsePermissionIDBody`, and similar helpers
- Permission and role handlers that separately bind URI params and JSON body before constructing command/query values

This creates several maintenance costs:

- Controller handlers must know the correct order of normalization, parsing, and mapping.
- Multi-source endpoints such as `URI + JSON` binding grow extra helper calls and branching.
- Adding a new endpoint requires developers to rediscover local conventions.
- Tests tend to target small helpers instead of endpoint-level input preparation behavior.

The proposed pattern makes the controller read as transport orchestration, while keeping input cleanup and application input construction in the feature-local HTTP boundary.

## Scope

In scope:

- HTTP request input preparation in `user-service/internal/features/*/transport/http`.
- Feature-local preparer functions that return application command/query values.
- Optional no-business-semantics binder composition support for multi-source request binding, such as `URI + JSON`.
- Tests for preparer behavior and compatibility with existing error semantics.
- Documentation updates to describe the convention.

Out of scope:

- Changing application command/query semantics.
- Moving business rules into transport.
- Changing domain validation or persistence behavior.
- Introducing `openspec/` or `docs/opsx/` artifacts.
- Adding `internal/shared` for this pattern.
- Moving user-service-specific input helpers into `common`.

## Success Criteria

- Controller handlers do not directly call `NormalizeXXX` or `ParseXXX`.
- Each handler has at most one visible post-bind input-preparation call.
- Multi-source endpoints use one bound request model and one preparer.
- Existing API error semantics remain compatible.
- Feature boundaries from `AGENTS.md` and `docs/ARCHITECTURE.md` remain intact.
- Relevant user-service tests pass after implementation.
