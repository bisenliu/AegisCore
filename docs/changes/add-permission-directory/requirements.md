## ADDED Requirements

### Requirement: Permission Directory Persistence
The system SHALL persist permissions as manually maintained HTTP endpoint permissions with uniqueness enforced by `http_method` and `path_template`.

#### Scenario: Create unique permission
- **WHEN** an administrator creates a permission with a valid name, module, HTTP method, and route template
- **THEN** the system stores the permission with the supplied metadata and preserves `http_method + path_template` as its unique identity

#### Scenario: Reject duplicate permission identity
- **WHEN** an administrator creates or updates a permission to use an existing `http_method + path_template` pair
- **THEN** the system rejects the operation with a conflict-style application error

#### Scenario: Normalize HTTP method identity
- **WHEN** an administrator submits a permission with a lower-case or mixed-case HTTP method
- **THEN** the system stores and compares the method using the canonical upper-case HTTP method

### Requirement: Permission Domain Validation
The system SHALL validate permission names, modules, HTTP methods, and route templates in domain or transport-neutral application validators before persistence.

#### Scenario: Reject unsupported HTTP method
- **WHEN** a permission command contains an unsupported HTTP method
- **THEN** the system rejects the command before writing to the permission store

#### Scenario: Reject invalid route template
- **WHEN** a permission command contains a path template that is empty, not absolute, or not under `/api/v1/`
- **THEN** the system rejects the command before writing to the permission store

### Requirement: System Permission Protection
The system SHALL protect system permissions from ordinary destructive changes.

#### Scenario: Reject system permission identity update
- **WHEN** an administrator updates a system permission and attempts to change its HTTP method or path template
- **THEN** the system rejects the operation and leaves the permission identity unchanged

#### Scenario: Allow system permission metadata update
- **WHEN** an administrator updates non-identity metadata for a system permission, such as name, description, module, or enabled state
- **THEN** the system applies the update without changing its protected identity

#### Scenario: Disable permission without deleting it
- **WHEN** an administrator disables a permission
- **THEN** the system marks the permission disabled without physically deleting the record

### Requirement: Manual System Permission Catalog
The system SHALL provide a manually maintained default system permission catalog in the permission application layer.

#### Scenario: Read default permissions
- **WHEN** application code requests the default permission catalog
- **THEN** the system returns explicit permission specifications with name, description, module, method, path template, and system flag

#### Scenario: Catalog does not grant roles
- **WHEN** the default permission catalog is read
- **THEN** the system does not create role bindings, user bindings, or authorization grants

### Requirement: Permission Management Use Cases
The system SHALL provide application command use cases to create permissions, update permissions, enable permissions, and disable permissions.

#### Scenario: Create permission through command use case
- **WHEN** a valid create permission command is executed
- **THEN** the system validates the input and delegates persistence through the permission application port

#### Scenario: Enable disabled permission
- **WHEN** an enable permission command is executed for an existing disabled permission
- **THEN** the system marks the permission enabled through the permission store

#### Scenario: Disable enabled permission
- **WHEN** a disable permission command is executed for an existing enabled permission
- **THEN** the system marks the permission disabled through the permission store

### Requirement: Permission Query Use Cases
The system SHALL provide application query use cases for permission list, permission detail, user effective permissions, and route diff.

#### Scenario: List permissions
- **WHEN** a permission list query is executed with pagination or filters
- **THEN** the system returns matching permissions without exposing transport or persistence implementation types

#### Scenario: Get permission detail
- **WHEN** a permission detail query is executed for an existing permission
- **THEN** the system returns the permission details

#### Scenario: Query user effective permissions
- **WHEN** a user effective permissions query is executed
- **THEN** the system returns effective permission data available from the permission application ports without introducing role management or role binding behavior

### Requirement: Route Discovery
The system SHALL discover authorizable Gin routes through a route catalog scanner port without importing Gin into the permission application layer.

#### Scenario: Exclude non-authorizable routes
- **WHEN** the route scanner reads registered service routes
- **THEN** it excludes `OPTIONS` routes, paths outside `/api/v1/`, and public auth session routes

#### Scenario: Return authorizable routes
- **WHEN** the route scanner reads registered protected service routes under `/api/v1/`
- **THEN** it returns discovered route method and path values for route diff comparison

#### Scenario: Discovery is read-only
- **WHEN** route discovery runs
- **THEN** the system does not create, update, delete, grant, or bind any permission, role, or user records

### Requirement: Permission Route Diff
The system SHALL expose route diff results containing only routes missing from stored permissions and stored permissions stale against discovered routes.

#### Scenario: Report missing routes
- **WHEN** an authorizable route exists in the discovered route catalog but no stored permission has the same method and path template
- **THEN** the route diff result includes that route in `MissingInPermissions`

#### Scenario: Report stale permissions
- **WHEN** a stored permission has a method and path template that no longer appears in the discovered authorizable route catalog
- **THEN** the route diff result includes that permission in `StalePermissions`

#### Scenario: Omit matching permissions
- **WHEN** a stored permission matches a discovered authorizable route by method and path template
- **THEN** the route diff result does not include it in either diff list

### Requirement: Permission HTTP API
The system SHALL expose permission management and query operations through feature-local Gin HTTP routes under `/api/v1/permissions` using the shared response envelope.

#### Scenario: Get route diff over HTTP
- **WHEN** a client sends `GET /api/v1/permissions/route-diff`
- **THEN** the service returns `MissingInPermissions` and `StalePermissions` using the shared HTTP response envelope

#### Scenario: HTTP DTOs map to application inputs
- **WHEN** a permission HTTP request is received
- **THEN** the controller maps feature-local request DTOs to application command or query inputs before invoking use cases

### Requirement: Permission Layer Boundaries
The system SHALL keep permission code within repository feature-layer dependency boundaries.

#### Scenario: Application boundary excludes transport and persistence libraries
- **WHEN** permission application packages are compiled
- **THEN** they do not import Gin, Ent, Redis, SQL, or HTTP response packages

#### Scenario: PostgreSQL adapter boundary excludes HTTP libraries
- **WHEN** permission `infrastructure/postgres` packages are compiled
- **THEN** they do not import Gin or HTTP response packages
