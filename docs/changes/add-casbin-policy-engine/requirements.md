## ADDED Requirements

### Requirement: Embedded Casbin model
The system SHALL define the Casbin authorization model in `user-service/internal/features/permission/infrastructure/casbin/model.conf` and SHALL load it at runtime using `go:embed`.

#### Scenario: Model loads from embedded file
- **WHEN** the permission Casbin engine is constructed
- **THEN** the engine uses the embedded `model.conf` content to create the Casbin model

#### Scenario: Model supports role-derived RBAC policies
- **WHEN** the embedded model evaluates a request
- **THEN** it matches `g(r.sub, p.sub)` and allows exact object/action matches or wildcard `*` object/action policies

### Requirement: Policy source of truth
The system SHALL build Casbin policies from `roles`, `permissions`, `user_roles`, and `role_permissions` records and SHALL NOT use a `casbin_rules` table as the business authority.

#### Scenario: User-role bindings become grouping policies
- **WHEN** policy is loaded for a user with an active role binding
- **THEN** the engine adds `g(user:<user_uuid>, role:<role_uuid>)`

#### Scenario: Role-permission bindings become permission policies
- **WHEN** policy is loaded for a role with an active permission binding
- **THEN** the engine adds `p(role:<role_uuid>, <path_template>, <http_method>)`

#### Scenario: Inactive records are excluded
- **WHEN** a role or permission has `active=false`
- **THEN** no policy derived from that inactive role or inactive permission is loaded

### Requirement: Super-admin wildcard policy
The system SHALL support a fixed super-admin role ID by loading `p(role:<super_admin_role_uuid>, *, *)` for that role.

#### Scenario: Super admin can access any route template and method
- **WHEN** a user is grouped into the configured super-admin role
- **THEN** `Enforce` allows any path template and HTTP method through the wildcard policy

#### Scenario: Super admin does not depend on role code
- **WHEN** the super-admin wildcard policy is configured
- **THEN** the engine uses the fixed role ID and does not require or query a `roles.code` field

### Requirement: In-memory enforcement
The system SHALL expose `Enforce(ctx, userID, pathTemplate, method)` and SHALL NOT query the database during each enforcement call.

#### Scenario: Matching policy allows access
- **WHEN** `Enforce` is called with a user ID, path template, and method that match the loaded grouping and permission policies
- **THEN** the engine returns an allow decision

#### Scenario: Missing policy denies access
- **WHEN** `Enforce` is called without a matching loaded grouping and permission policy
- **THEN** the engine returns a deny decision

#### Scenario: Enforcement avoids request-time database access
- **WHEN** `Enforce` evaluates a loaded policy
- **THEN** it uses only the in-memory Casbin enforcer state

### Requirement: Full reload safety
The system SHALL support full policy reload and SHALL preserve the current policy when reload fails.

#### Scenario: Successful reload replaces policy
- **WHEN** `Reload` successfully reads RBAC data and builds a complete new enforcer
- **THEN** subsequent `Enforce` calls use the newly loaded policy

#### Scenario: Failed reload keeps previous policy
- **WHEN** `Reload` fails while reading data or building policy
- **THEN** the previous in-memory policy remains active and no partially updated policy is exposed

### Requirement: Fail-closed initialization
The system SHALL deny enforcement decisions by default when initial enforcer construction fails.

#### Scenario: Initial policy construction fails
- **WHEN** the engine cannot initialize its model or load initial policies
- **THEN** `Enforce` denies access until a later successful reload publishes a valid policy
