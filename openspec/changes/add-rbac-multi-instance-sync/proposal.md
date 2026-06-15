## Why

RBAC policy is currently loaded into each service instance's in-memory Casbin enforcer, but policy-changing role and permission writes do not have a cross-instance propagation mechanism. In multi-instance deployments this can leave instances authorizing with stale policy until restart or manual reload, so policy changes need a lightweight Redis-based notification and compensation path.

## What Changes

- Add Redis-backed RBAC policy version tracking owned by the permission feature infrastructure.
- Add a policy change notification flow that reloads the local Casbin enforcer after successful RBAC changes, increments the Redis policy version, and publishes a Redis Pub/Sub refresh message.
- Add a policy version watcher that subscribes to Redis Pub/Sub messages and performs full policy reloads on other instances.
- Add periodic Redis version checks so missed Pub/Sub messages are compensated by detecting newer policy versions.
- Add English logs for policy version changes, refresh success, refresh failure, Redis publish failures, and version mismatch detection.
- Preserve the existing request-time authorization model: each request authorizes against the in-memory Enforcer only and does not access Redis.
- Keep policy synchronization independent of `roles.code`; Casbin subjects continue to use `role:<role_uuid>`.

## Capabilities

### New Capabilities

- `rbac-multi-instance-sync`: Redis policy version notification and compensation behavior for synchronizing in-memory RBAC policy across user-service instances.

### Modified Capabilities

None.

## Impact

- Affects `user-service/internal/features/permission`, especially Casbin policy reload coordination and new Redis infrastructure under `permission/infrastructure/redis`.
- Affects role and permission command success paths that mutate RBAC policy source data.
- Uses the existing named Redis client resource and Redis Pub/Sub primitives; does not add Kafka, RabbitMQ, NATS, Redis Stream, eventbus, outbox, Casbin rule persistence, or new database tables.
- Does not change HTTP API contracts, RBAC authority data sources, per-request authorization dependencies, or Casbin subject semantics.
