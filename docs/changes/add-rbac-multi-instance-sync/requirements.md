## ADDED Requirements

### Requirement: Policy changes publish distributed refresh signals
The system SHALL refresh the local RBAC policy and publish a Redis-backed policy version notification after successful role or permission changes that affect Casbin policy source data.

#### Scenario: Role permission binding changes publish refresh
- **WHEN** an instance successfully adds, removes, or replaces role-permission bindings
- **THEN** the instance reloads its local in-memory RBAC policy, increments the Redis policy version, and publishes a Redis Pub/Sub refresh message with the new version

#### Scenario: User role binding changes publish refresh
- **WHEN** an instance successfully adds, removes, or replaces user-role bindings
- **THEN** the instance reloads its local in-memory RBAC policy, increments the Redis policy version, and publishes a Redis Pub/Sub refresh message with the new version

#### Scenario: Role or permission active state changes publish refresh
- **WHEN** an instance successfully changes role active state, permission active state, or permission route identity fields used by policy loading
- **THEN** the instance reloads its local in-memory RBAC policy, increments the Redis policy version, and publishes a Redis Pub/Sub refresh message with the new version

### Requirement: Instances reload on remote policy notifications
The system SHALL subscribe to Redis policy refresh messages and perform a full RBAC policy reload when a newer policy version is observed.

#### Scenario: Remote instance receives newer version
- **WHEN** an instance receives a Redis Pub/Sub policy refresh message with a version newer than its locally applied policy version
- **THEN** it performs a full Casbin policy reload from the authoritative PostgreSQL RBAC data

#### Scenario: Reload succeeds after notification
- **WHEN** a remote notification-triggered policy reload succeeds
- **THEN** the instance updates its locally applied policy version and logs the refresh success with the policy version

#### Scenario: Reload fails after notification
- **WHEN** a remote notification-triggered policy reload fails
- **THEN** the instance preserves the last good in-memory policy and logs the refresh failure with the policy version and error

### Requirement: Periodic version checks compensate for missed notifications
The system SHALL periodically compare the Redis RBAC policy version with the locally applied version and reload policy when Redis indicates a newer version.

#### Scenario: Pub/Sub message is missed
- **WHEN** an instance misses a Redis Pub/Sub refresh message and a later periodic check reads a Redis policy version newer than its local version
- **THEN** it logs the version mismatch and performs a full Casbin policy reload

#### Scenario: Versions are already synchronized
- **WHEN** a periodic check reads a Redis policy version that is not newer than the local version
- **THEN** the instance does not reload policy

### Requirement: Redis coordination failures do not block local RBAC changes
The system SHALL treat Redis policy version and Pub/Sub failures as coordination failures that do not roll back successful RBAC data changes or successful local policy reloads.

#### Scenario: Redis unavailable after local reload
- **WHEN** an instance successfully applies an RBAC data change and successfully reloads local policy but Redis version increment or publish fails
- **THEN** the RBAC data change remains committed, the local enforcer remains refreshed, and the instance logs the Redis coordination error

#### Scenario: Redis unavailable during periodic check
- **WHEN** an instance cannot read the Redis policy version during a periodic check
- **THEN** it keeps using the current in-memory enforcer and logs the Redis check failure

### Requirement: Request-time authorization remains in-memory only
The system SHALL continue to authorize protected HTTP requests using only the in-memory Casbin enforcer loaded in the current instance.

#### Scenario: Protected request is authorized
- **WHEN** a protected HTTP request reaches the RBAC middleware
- **THEN** the authorization decision is made against the current in-memory enforcer without reading Redis or publishing Redis messages

#### Scenario: Policy is stale until reload
- **WHEN** an instance has not yet received or detected a newer policy version
- **THEN** request-time authorization continues using the last successfully loaded in-memory policy until a reload succeeds

### Requirement: Policy synchronization uses UUID-based Casbin subjects
The system SHALL keep RBAC policy synchronization independent of role code fields and continue using UUID-based Casbin subjects.

#### Scenario: Role policy is loaded
- **WHEN** RBAC policy is loaded during startup, notification refresh, or periodic compensation
- **THEN** role subjects use `role:<role_uuid>` and user subjects use `user:<user_uuid>` without requiring `roles.code`

### Requirement: Policy sync behavior is observable through English logs
The system SHALL log policy synchronization events with English messages and stable snake_case fields.

#### Scenario: Policy version changes
- **WHEN** an instance increments, observes, or applies an RBAC policy version
- **THEN** it logs the event with fields such as `policy_version`, `local_policy_version`, `remote_policy_version`, and `instance_id`

#### Scenario: Refresh fails
- **WHEN** a local, notification-triggered, or periodic policy refresh fails
- **THEN** it logs the failure with an English message, the relevant policy version fields, and the error

### Requirement: Redis policy keys use the shared key builder
The system SHALL construct RBAC policy Redis keys and Pub/Sub channels through `common/runtime/rediskey` from a feature-local permission Redis key catalog.

#### Scenario: Policy version key is constructed
- **WHEN** the permission Redis adapter needs the RBAC policy version key
- **THEN** it builds the key with the shared Redis key builder using the app namespace and `rbac` scope instead of hardcoded colon-delimited literals

#### Scenario: Policy refresh channel is constructed
- **WHEN** the permission Redis adapter subscribes to or publishes the RBAC policy refresh channel
- **THEN** it builds the channel name with the shared Redis key builder using the app namespace and `rbac` scope instead of hardcoded colon-delimited literals
