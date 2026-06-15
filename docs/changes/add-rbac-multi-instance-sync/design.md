## Context

User-service RBAC authorization is enforced by an in-memory Casbin enforcer owned by the permission feature. The enforcer is built from PostgreSQL-backed role, user-role, permission, and role-permission state and is used by the Gin RBAC middleware for every protected request.

Today the enforcer reloads at startup, but policy-changing role and permission writes do not coordinate policy refreshes across service instances. In a multi-instance deployment, one instance can mutate RBAC source data while another instance continues to authorize with stale in-memory policy.

The repository constraints require feature-local ownership for RBAC authorization behavior, no new MQ/eventbus/outbox design, no Casbin policy table, no request-time Redis lookup, and no dependency on `roles.code` for Casbin policy subjects.

## Goals / Non-Goals

**Goals:**

- Refresh the local Casbin enforcer after successful RBAC policy source mutations.
- Notify other user-service instances about policy changes through Redis Pub/Sub.
- Store a monotonically increasing Redis policy version so instances can detect missed Pub/Sub messages.
- Periodically compare local and Redis policy versions and compensate by performing a full policy reload when Redis is newer.
- Keep request-time authorization served only from the in-memory Casbin enforcer.
- Log policy version changes, refresh success, refresh failure, publish failure, and version mismatch detection with English messages and stable snake_case fields.

**Non-Goals:**

- No Kafka, RabbitMQ, NATS, Redis Stream, generic eventbus, or outbox implementation.
- No `casbin_rules` table or database persistence of Casbin adapter policy.
- No change to PostgreSQL as the authoritative RBAC source of truth.
- No menu permission, tenant isolation, or audit log feature.
- No request-time Redis access in the RBAC authorization middleware or authorization application service.
- No dependency on `roles.code`; Casbin subjects remain UUID based.

## Decisions

### Feature-local Redis policy sync adapter

Add the Redis policy version and watcher implementation under `user-service/internal/features/permission/infrastructure/redis`. The permission feature owns RBAC authorization and Casbin policy reload behavior, while Redis is an infrastructure adapter for distributed coordination.

Alternative considered: place the watcher in `common/runtime` or `internal/providers`. This was rejected because RBAC policy version keys, Pub/Sub channel semantics, and reload decisions are permission-feature behavior rather than reusable runtime primitives.

### Policy reload coordinator as the command-facing abstraction

Introduce a small permission application-level port/service for policy refresh notification, then inject it into role and permission command services that mutate policy source data. After a successful PostgreSQL mutation, the command path triggers the coordinator to reload the local Casbin engine first, then increment the Redis policy version and publish a refresh message.

Local reload happens before Redis publication so the writing instance does not announce a version it failed to apply locally. Redis increment or publish failure is logged but does not roll back the already successful RBAC write or local reload.

Alternative considered: have PostgreSQL adapters publish directly after writes. This was rejected because adapters must not own business orchestration or HTTP-facing policy refresh semantics.

### Redis version key plus Pub/Sub channel

Use one Redis string key for the latest RBAC policy version and one Pub/Sub channel for refresh notifications. The key and channel names live in the feature Redis adapter as a storage contract and MUST be built through `common/runtime/rediskey` using the app namespace and an `rbac` scope, matching the existing feature-local key catalog pattern used by auth Redis infrastructure.

The version increment is the authoritative signal for ordering. Pub/Sub messages carry the new version and an instance identifier so receivers can log origin and optionally ignore duplicate self-originated messages if needed. Receivers still compare message version against local applied version before reloading.

Alternative considered: use Redis Streams for durable delivery. This was rejected because the change explicitly excludes Redis Stream and because periodic version checks provide acceptable compensation for Pub/Sub loss without adding consumer group lifecycle complexity.

### Full reload on notification or version mismatch

Receiving a newer Pub/Sub version or detecting a newer Redis version during periodic checks triggers a full Casbin reload using the existing `Engine.Reload` path. The current enforcer replacement semantics are preserved: successful reload atomically replaces the in-memory enforcer, while failure preserves the last good policy.

Alternative considered: incrementally patch Casbin policies. This was rejected because role activation, permission activation, route template changes, user-role bindings, and role-permission bindings can interact; full reload is simpler and consistent with the existing loader.

### Periodic compensation loop with Fx lifecycle

Register the watcher with Fx lifecycle so subscription and periodic checks start with the service and stop on shutdown. The loop uses context cancellation, clean Pub/Sub close behavior, and bounded check intervals from configuration or conservative defaults.

Alternative considered: a naked goroutine started from a constructor. This was rejected because long-running background work must have explicit lifecycle management.

### Redis failures are non-fatal after startup

If Redis is temporarily unavailable during policy notification or periodic checks, the current instance still applies local policy reloads and logs dependency failures. Other instances may remain stale until Redis recovers or another change publishes a later version; version mismatch logs make the condition observable.

Alternative considered: fail RBAC write operations when Redis notification fails. This was rejected because PostgreSQL remains the RBAC source of truth and local policy reload should not be blocked by a transient coordination dependency.

## Risks / Trade-offs

- Redis Pub/Sub is lossy -> periodic version checks compare Redis and local versions and perform full reload on mismatch.
- Redis unavailable after local reload -> the writing instance has fresh local policy, logs publish/version errors, and other instances refresh when Redis recovers or a later version is observed.
- Multiple rapid RBAC changes can trigger repeated reloads -> receivers compare versions and can coalesce in-flight refreshes or skip stale versions during implementation.
- Reload failure on a receiving instance preserves stale policy -> the existing enforcer behavior keeps the last good policy and logs refresh failure with version context for diagnosis.
- Version increment after local reload can leave Redis version unchanged if Redis fails -> this favors local correctness and observability over distributed strictness without introducing an outbox.

## Migration Plan

1. Add the feature-local Redis policy sync components and Fx lifecycle wiring.
2. Wire role and permission command services to call the refresh coordinator after successful policy-affecting writes.
3. Add tests for local reload behavior, Redis publish/version failure tolerance, Pub/Sub-triggered reload, periodic compensation, and request-time Redis isolation.
4. Deploy normally; existing instances without the watcher continue to use startup-loaded policy until replaced.
5. Roll back by deploying the previous version; Redis policy version keys and channels are ephemeral coordination state and require no schema rollback.

## Open Questions

- The exact periodic check interval can use a conservative default initially; making it configurable can be included if the existing configuration pattern has an appropriate place.
- Whether seed workflows should trigger policy sync immediately depends on whether RBAC seed is wired into runtime execution during implementation.
