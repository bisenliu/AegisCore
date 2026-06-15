# Design

## Current State

The current HTTP flow is:

1. Middleware and route registration enter a feature controller.
2. Controller calls `binding.BindOrAbort`.
3. Controller calls one or more feature-local helpers, commonly named `NormalizeXXX` and `ParseXXX`.
4. Controller manually constructs an application command/query.
5. Controller calls the application use case and maps the result to response DTOs.

This is visible in:

- `user/transport/http/controller.go`: list and get handlers call both normalize and parse helpers.
- `auth/transport/http/controller.go`: login, refresh, and change-password handlers normalize request values after binding.
- `permission/transport/http/controller.go`: list, effective permissions, and ID-based handlers parse IDs after binding.
- `role/transport/http/controller.go`: role binding endpoints split URI parsing and body parsing across multiple private helpers.

The existing separation is mostly correct, but controller readability suffers because the controller is responsible for the sequence of input cleanup operations.

## Target Shape

Each endpoint should have one request model and one preparer.

Single-source example:

```go
req := ListUsersRequest{}
if !binding.BindOrAbort(ctl.validator, c, &req, binding.QueryBinder) {
	return
}

query, err := prepareListUsersQuery(req)
if err != nil {
	response.Fail(c, err)
	return
}
```

Multi-source example:

```go
req := ReplaceUserRolesHTTPRequest{}
if !binding.BindOrAbort(ctl.validator, c, &req, binding.Compose(binding.URIBinder, binding.JSONBinder)) {
	return
}

cmd, err := prepareReplaceUserRolesCommand(req)
if err != nil {
	response.Fail(c, err)
	return
}
```

## Responsibility Model

| Area | Responsibility |
|---|---|
| `binding.BindOrAbort` | Bind URI/query/form/JSON values, apply `SetDefaults()`, run struct tag validation, run request `Validate()`, write binding or validation failure responses. |
| `transport/http` preparer | Trim text, lowercase HTTP-specific fields where needed, normalize pagination defaults, strip Bearer prefixes, parse UUIDs/cursors, merge bound request fields, and construct application command/query values. |
| `application/validators` | Transport-neutral input rules shared by application use cases, such as role or permission field normalization. |
| `domain` | Domain invariants and value-object validation. |
| `infrastructure` | Database, Redis, Casbin, and external adapter behavior. |

Preparers must not query stores, call application use cases, perform authorization, emit responses, or log business outcomes.

## File Layout

Use feature-local files:

- `user-service/internal/features/user/transport/http/input.go`
- `user-service/internal/features/auth/transport/http/input.go`
- `user-service/internal/features/role/transport/http/input.go`
- `user-service/internal/features/permission/transport/http/input.go`

Keep request DTO definitions in `request.go`. For multi-source endpoint DTOs, either:

- add endpoint-specific HTTP request models in `request.go`, or
- place them near preparers in `input.go` when they are implementation-only and should not be exposed as Swagger body models.

Prefer `request.go` for any DTO that appears in Swagger documentation.

## Naming Rules

Use names that describe the application input being produced:

- `prepareListUsersQuery`
- `prepareCreateUserCommand`
- `prepareGetUserByIDQuery`
- `prepareLoginCommand`
- `prepareRefreshTokenCommand`
- `prepareChangePasswordCommand`
- `prepareListRolesQuery`
- `prepareReplaceUserRolesCommand`
- `prepareRolePermissionCommand`
- `prepareListPermissionsQuery`
- `prepareSetPermissionActiveCommand`

Avoid generic names such as `ValidateRequest`, `BuildInput`, or `HandleParams`.

## Preparer Rules

Each preparer should follow this internal order:

1. Text cleanup: `strings.TrimSpace`, optional lowercase for fields that already require lowercase behavior.
2. Default and pagination normalization: `pagination.NormalizePageSize`, derived `Limit`.
3. Token cleanup: `commonauth.StripBearerPrefix` for HTTP token inputs.
4. Identifier parsing: UUID and cursor conversion with resource-specific error messages.
5. Application input construction: return command/query values.

If a preparer must return an error, it should return the same public error category and message as the current helper path.

## Multi-Source Binding

Endpoints that currently perform separate URI and JSON binding should move to one request model. A no-business-semantics binder composer may be introduced:

```go
func Compose(binders ...Binder) Binder {
	return func(c *gin.Context, dst any) error {
		for _, binder := range binders {
			if err := binder(c, dst); err != nil {
				return err
			}
		}
		return nil
	}
}
```

This belongs in `common/http/binding` only because it is generic binder orchestration with no user-service semantics.

If adding to `common` feels too broad for the first iteration, use feature-local private binders first and promote only after at least two features need the same exact primitive.

## Feature Mapping

### User

- `ListUsers`: combine `NormalizeListUsers`, `ParseListCursor`, and `userquery.ListUsersQuery` construction.
- `CreateUser`: combine `NormalizeCreateUser` and `usercommand.CreateUserCommand` construction.
- `GetByUserID`: combine `ParseUserID` and `userquery.GetUserByIDQuery` construction.

### Auth

- `LoginUser`: combine `NormalizeLogin` and `authcommand.LoginCommand` construction.
- `RefreshToken`: combine `NormalizeRefresh` and `authcommand.RefreshTokenCommand` construction.
- `ChangePassword`: bind JSON and Authorization header into one request or pass the header explicitly to the preparer, then construct `authcommand.ChangePasswordCommand`.

The client audit context built from `ClientIP` and `User-Agent` should remain visible in the controller because it modifies request context, not just input DTO shape.

### Role

Role has the highest payoff because many endpoints currently split URI parsing and body parsing.

- `ListRoles`: prepare `rolequery.ListRolesQuery`.
- `CreateRole`: prepare `rolecommand.CreateRoleCommand`.
- `UpdateRole`: use URI + JSON request, prepare `rolecommand.UpdateRoleCommand`.
- `SetRoleStatus`: use URI + JSON request, prepare `rolecommand.SetRoleActiveCommand`.
- `ListUserRoles`: prepare `rolequery.UserRolesQuery`.
- `ReplaceUserRoles`: use URI + JSON request, prepare `rolecommand.ReplaceUserRolesCommand`.
- `AddUserRole`: use URI + JSON request, prepare `rolecommand.UserRoleCommand`.
- `RemoveUserRole`: prepare `rolecommand.UserRoleCommand` from URI.
- `ListRolePermissions`: prepare `rolequery.RolePermissionsQuery`.
- `ReplaceRolePermissions`: use URI + JSON request, prepare `rolecommand.ReplaceRolePermissionsCommand`.
- `AddRolePermission`: use URI + JSON request, prepare `rolecommand.RolePermissionCommand`.
- `RemoveRolePermission`: prepare `rolecommand.RolePermissionCommand` from URI.

### Permission

- `ListPermissions`: prepare `permissionquery.ListPermissionsQuery`.
- `CreatePermission`: prepare `permissioncommand.CreatePermissionCommand`.
- `GetPermission`: prepare `permissionquery.GetPermissionQuery`.
- `UpdatePermission`: use URI + JSON request, prepare `permissioncommand.UpdatePermissionCommand`.
- `EnablePermission` and `DisablePermission`: share ID preparer and construct `permissioncommand.SetPermissionActiveCommand`.
- `ListUserEffectivePermissions`: prepare `permissionquery.UserEffectivePermissionsQuery`.
- `GetRouteDiff`: no request input; no preparer required.

## Error Compatibility

Keep these mappings stable:

- Invalid user UUID: `messages.InvalidUserID`.
- Invalid role UUID: `messages.InvalidRole`.
- Invalid permission UUID: `messages.InvalidPermission`.
- Invalid list cursor: continue using the same resource-specific invalid message as today.
- Empty login credentials: keep unauthenticated semantics.
- Missing or blank refresh/change-password token: keep token invalid or unauthenticated semantics as currently exposed by the endpoint.
- Empty password after trim: keep validation failure semantics.

## Testing Strategy

Add focused preparer tests per feature:

- valid input returns the expected command/query.
- whitespace trimming is applied.
- page size normalization produces expected `PageSize` and `Limit`.
- empty cursor returns nil cursor.
- invalid cursor or UUID returns the current public error.
- Bearer token cleanup preserves valid token values and rejects blank token inputs.
- multi-source request models produce expected commands.

Keep existing controller tests for response behavior. Controller tests should not need to assert the sequence of normalize/parse helpers after refactor.

## Maintainability Guardrails

- A preparer is allowed to depend on `application` command/query packages and domain value types.
- A preparer must not depend on Gin, Ent, Redis, SQL, or infrastructure packages.
- A preparer should stay pure where possible. It should not mutate global state, call stores, or emit responses.
- Request DTOs remain transport-specific and must not leak into application services.
- Do not introduce `internal/shared` for this pattern.
- Do not move feature-specific preparers into `common`.
- Do not add speculative gRPC, event, or outbox abstractions while implementing this change.
