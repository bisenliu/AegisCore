## ADDED Requirements

### Requirement: Explicit RBAC Seed Entry Point
The system SHALL provide an explicit RBAC seed entry point that can be run independently of the HTTP server lifecycle.

#### Scenario: Seed runs outside HTTP startup
- **WHEN** an operator runs `aegiscore-user-services rbac seed`
- **THEN** the system writes RBAC seed data without starting the HTTP server

#### Scenario: HTTP server does not seed implicitly
- **WHEN** an operator runs `aegiscore-user-services serve`
- **THEN** the system MUST NOT automatically seed RBAC roles, permissions, or bindings as part of HTTP startup

### Requirement: RBAC Seed Makefile Wrapper
The system SHALL provide a `make seed-rbac` wrapper for the RBAC seed command.

#### Scenario: Make wrapper invokes seed
- **WHEN** an operator runs `make seed-rbac`
- **THEN** the system invokes the RBAC seed command with the configured user-service configuration path

### Requirement: Catalog Managed System Roles
The system SHALL define default system roles in `user-service/internal/features/role/application/catalog/` using stable role IDs.

#### Scenario: Super admin role is present in catalog
- **WHEN** the role catalog is loaded
- **THEN** it includes the super admin role with role ID `00000000-0000-0000-0000-000000000001`, system flag enabled, and a human-readable name and description

### Requirement: Catalog Managed System Permissions
The system SHALL define default system permissions in `user-service/internal/features/permission/application/catalog/` using stable permission IDs and HTTP route identity.

#### Scenario: User list permission is present in catalog
- **WHEN** the permission catalog is loaded
- **THEN** it includes a system permission for `GET /api/v1/users` with a stable permission ID

#### Scenario: User create permission is present in catalog
- **WHEN** the permission catalog is loaded
- **THEN** it includes a system permission for `POST /api/v1/users` with a stable permission ID

### Requirement: Catalog Managed System Role Permissions
The system SHALL define default system role permission bindings in code catalog using stable role ID and permission ID pairs.

#### Scenario: Super admin default permissions are declared
- **WHEN** the role permission catalog is loaded
- **THEN** it includes default bindings from the super admin role to each catalog permission intended for super admin access

### Requirement: Idempotent System Role Upsert
The RBAC seed command SHALL idempotently upsert system roles by `role_id`.

#### Scenario: System role is inserted
- **WHEN** a catalog role does not exist in the roles table
- **THEN** seed inserts it with catalog name, description, active state, system flag, and timestamps

#### Scenario: System role is updated
- **WHEN** a catalog role already exists by `role_id`
- **THEN** seed updates catalog-managed fields and preserves idempotent repeat execution

### Requirement: Idempotent System Permission Upsert
The RBAC seed command SHALL idempotently upsert system permissions by stable `permission_id` or by unique `http_method + path_template` identity.

#### Scenario: System permission is inserted
- **WHEN** a catalog permission does not exist in the permissions table
- **THEN** seed inserts it with catalog permission ID, name, description, module, HTTP method, path template, active state, system flag, and timestamps

#### Scenario: System permission is updated by permission ID
- **WHEN** a catalog permission already exists by `permission_id`
- **THEN** seed updates catalog-managed fields and preserves idempotent repeat execution

#### Scenario: System permission is matched by route identity
- **WHEN** a catalog permission does not match by `permission_id` but matches an existing permission by `http_method + path_template`
- **THEN** seed updates the existing permission as the system catalog entry instead of creating a duplicate route permission

### Requirement: Preserve Disabled System Entries By Default
The RBAC seed command SHALL NOT reactivate existing system roles or system permissions whose `active` value is `false` unless explicitly requested.

#### Scenario: Disabled system role remains disabled
- **WHEN** an existing catalog system role has `active=false` and seed is run without `--reactivate-system`
- **THEN** the role remains disabled after seed completes

#### Scenario: Disabled system permission remains disabled
- **WHEN** an existing catalog system permission has `active=false` and seed is run without `--reactivate-system`
- **THEN** the permission remains disabled after seed completes

#### Scenario: Reactivate option restores catalog entries
- **WHEN** seed is run with `--reactivate-system`
- **THEN** existing catalog system roles and permissions are updated to the active state declared by the catalog

### Requirement: System Role Permission Binding Completion
The RBAC seed command SHALL add missing catalog-defined system role permission bindings.

#### Scenario: Missing binding is added
- **WHEN** a catalog role permission binding does not exist in the database
- **THEN** seed creates the role permission binding

#### Scenario: Existing binding is kept
- **WHEN** a catalog role permission binding already exists in the database
- **THEN** seed leaves the binding present and completes successfully

### Requirement: System Binding Synchronization Option
The RBAC seed command SHALL provide `--sync-system-bindings` to precisely synchronize catalog-managed system role permission bindings.

#### Scenario: Default seed does not remove extra bindings
- **WHEN** a system role has an extra permission binding not present in the catalog and seed is run without `--sync-system-bindings`
- **THEN** seed does not remove the extra binding

#### Scenario: Sync option removes extra catalog-scope bindings
- **WHEN** a system role has an extra permission binding not present in the catalog and seed is run with `--sync-system-bindings`
- **THEN** seed removes the extra binding for the catalog-managed system role

### Requirement: No Automatic Deletion Of Removed Catalog Entries
The RBAC seed command SHALL NOT automatically delete roles or permissions that were removed from catalog.

#### Scenario: Removed role catalog entry remains in database
- **WHEN** a role exists in the database but is no longer present in the role catalog
- **THEN** seed does not delete that role

#### Scenario: Removed permission catalog entry remains in database
- **WHEN** a permission exists in the database but is no longer present in the permission catalog
- **THEN** seed does not delete that permission

### Requirement: No Automatic Custom Role Authorization
The RBAC seed command SHALL NOT automatically grant catalog permissions to non-system custom roles.

#### Scenario: Custom role is not granted new system permission
- **WHEN** a new catalog system permission is added and seed is run
- **THEN** seed does not bind that permission to ordinary custom roles

### Requirement: Explicit Super Admin Assignment
The system SHALL provide `aegiscore-user-services rbac assign-super-admin --user-id <uuid>` to bind the super admin role to a specified user.

#### Scenario: Super admin role is assigned to user
- **WHEN** an operator runs `aegiscore-user-services rbac assign-super-admin --user-id <uuid>` for an existing user
- **THEN** the system binds that user to the catalog super admin role idempotently

#### Scenario: Seed does not assign super admin automatically
- **WHEN** an operator runs `aegiscore-user-services rbac seed`
- **THEN** the system does not bind any user to the super admin role unless the assign command is run separately

### Requirement: System Permission Update Workflow
The system SHALL support adding or changing protected-route permissions through catalog update, seed execution, route-diff validation, and policy reload.

#### Scenario: New system permission workflow
- **WHEN** a protected route is added or changed
- **THEN** an operator can update the permission catalog, run `make seed-rbac`, verify route-diff MissingInPermissions and StalePermissions, and execute or trigger policy reload without granting ordinary custom roles automatically

### Requirement: System Role Update Workflow
The system SHALL support adding or changing system roles through role catalog update, optional default role-permission catalog update, seed execution, and policy reload.

#### Scenario: New system role workflow
- **WHEN** a system role is added to the role catalog
- **THEN** an operator can update DefaultRoles and DefaultRolePermissions, run `make seed-rbac`, and execute or trigger policy reload without binding users automatically

### Requirement: Operational Custom Role Workflow Remains Explicit
The system SHALL keep operational custom role creation, permission binding, and user binding explicit through role management workflows.

#### Scenario: Custom role is managed through role APIs
- **WHEN** an operator creates an ordinary custom role
- **THEN** the operator uses role creation, role permission binding, and user role binding APIs rather than RBAC seed automation

### Requirement: Operational Permission Creation Rules
The system SHALL keep operational custom permission creation constrained to real protected routes and unique route identity.

#### Scenario: Custom permission validates route identity
- **WHEN** operational permission creation is enabled and a permission is created
- **THEN** the system requires unique `http_method + path_template` and the path template corresponds to a real protected route rather than public auth, health, Swagger, or OPTIONS routes

### Requirement: Permission Deprecation Workflow
The system SHALL support permission deprecation by route-diff review and explicit deactivation rather than automatic seed deletion.

#### Scenario: Stale permission is deprecated
- **WHEN** route-diff reports StalePermissions
- **THEN** an operator reviews whether the route is deprecated or changed, sets the permission `active=false` when appropriate, and triggers policy reload

### Requirement: User Service Tests Pass
The implementation SHALL keep user-service tests passing.

#### Scenario: User-service test suite succeeds
- **WHEN** `go test ./...` is run under `user-service/`
- **THEN** the test suite passes
