## ADDED Requirements

### Requirement: RBAC command and query regression coverage
The system SHALL provide unit test coverage for role and permission command/query behavior without changing business behavior or database schema.

#### Scenario: Role command and query behavior is covered
- **WHEN** role command and query use cases are exercised by tests
- **THEN** role lifecycle, user-role binding, role-permission binding, disabled role, user unbinding, role-permission unbinding, and role query behavior are validated through deterministic unit tests

#### Scenario: Permission command and query behavior is covered
- **WHEN** permission command and query use cases are exercised by tests
- **THEN** permission lifecycle, disabled permission, effective permission lookup, and route diff query behavior are validated through deterministic unit tests

#### Scenario: Application layer remains transport and infrastructure independent
- **WHEN** RBAC command/query tests and layer checks are run
- **THEN** application packages MUST NOT import Gin, Ent, Redis clients, or SQL infrastructure packages

### Requirement: RBAC authorization regression coverage
The system SHALL provide tests for Casbin policy loading, enforcement, reload behavior, and Gin HTTP authorization middleware decisions.

#### Scenario: Casbin uses role identifiers for policy subjects
- **WHEN** authorization policies are loaded for roles
- **THEN** Casbin enforcement MUST use `role_id` as the policy subject and MUST NOT require `roles.code` in role query results

#### Scenario: Authorization denies inactive or missing assignments
- **WHEN** a request is made by a user with no roles, a disabled role, a disabled permission, a removed user-role binding, or a removed role-permission binding
- **THEN** the RBAC middleware MUST deny access according to the existing authorization response contract

#### Scenario: Super administrator wildcard is honored
- **WHEN** a user has the `super_admin` wildcard policy applicable to the requested route
- **THEN** the RBAC middleware MUST authorize the request without requiring a route-specific permission binding

#### Scenario: Enforcer reload updates decisions
- **WHEN** role, permission, or binding state changes and the Casbin enforcer reload path is exercised
- **THEN** subsequent authorization decisions MUST reflect the reloaded policy state

### Requirement: Permission route diff remains read-only
The system SHALL keep permission route diff behavior limited to automatic route discovery and difference reporting.

#### Scenario: Route diff reports missing and extra permissions
- **WHEN** registered HTTP routes are compared with persisted formal permissions
- **THEN** the route diff result MUST report discovered differences without mutating formal permissions

#### Scenario: Route diff does not bind roles
- **WHEN** route diff execution discovers a route that is not represented by a formal permission
- **THEN** the system MUST NOT create role-permission bindings or otherwise assign discovered routes to roles

### Requirement: RBAC API documentation is reproducible
The system SHALL keep Swagger annotations and generated Swagger artifacts aligned with the implemented role, permission, and RBAC-protected HTTP APIs.

#### Scenario: Swagger artifacts match RBAC endpoints
- **WHEN** the documented Swagger generation command is run after RBAC annotation updates
- **THEN** generated Swagger artifacts MUST describe the current role and permission APIs, request and response models, and authorization behavior without manual edits to generated files

### Requirement: RBAC architecture documentation is current
The system SHALL document role and permission as implemented bounded features with stable feature boundaries and testing guidance.

#### Scenario: Architecture docs describe implemented RBAC boundaries
- **WHEN** `AGENTS.md` and `docs/ARCHITECTURE.md` are read
- **THEN** they MUST describe role and permission feature ownership, application ports, transport DTO boundaries, PostgreSQL adapter boundaries, and permission lookup integration as implemented behavior rather than skeleton placeholders

#### Scenario: Development and testing docs include RBAC regression commands
- **WHEN** `docs/DEVELOPMENT.md` or `docs/TESTING.md` is read
- **THEN** it MUST describe the RBAC-related test scope, the relevant acceptance commands, route diff read-only expectation, and the fact that `roles.code` is not required for Casbin authorization
