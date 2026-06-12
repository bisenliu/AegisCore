# Design

## Overview

This change defines event boundary ownership without introducing a broker or delivery runtime.

Future event code should follow this responsibility split:

```text
domain/events
  -> pure domain facts only, optional per feature

application/command|query|validators
  -> business orchestration and feature-owned ports

infrastructure/postgres|redis
  -> service-owned persistence resources

infrastructure/consumers
  -> feature-local inbound event consumer adapter, only when a real consumer exists

internal/integration/events
  -> external broker protocol adapter, envelope/topic mapping, producer/consumer wrapper

common/runtime/eventbus
  -> cross-service stable eventbus runtime primitive, only after real multi-service reuse exists

outbox
  -> delivery reliability boundary requiring separate transaction/storage/worker design
```

The short version: `integration/events` speaks to the outside event system; feature `infrastructure/consumers` translates normalized inbound events into feature application calls; feature application owns business decisions; `common` is only for stable, cross-service runtime primitives.

## Boundary Responsibilities

### `user-service/internal/integration/events`

`integration/events` is the anti-corruption boundary for external event systems. It may eventually contain:

- Broker-specific producer and consumer wrappers.
- Topic, subject, stream or routing-key mapping.
- External envelope, headers, partition key, trace metadata and serialization mapping.
- Broker error, retry result, nack/ack and deserialization error normalization.
- Adapter implementations for feature-owned application ports when a concrete feature needs to publish events.

It must not contain:

- Feature business orchestration.
- Application command/query implementations.
- Domain policy decisions beyond simple external-to-internal value mapping.
- Ent, SQL, Redis store access or outbox persistence.
- Gin controllers, HTTP response envelopes or route registration.
- Generic interfaces created only for adapter convenience.

Producer code belongs here only when there is a real external broker and a feature-owned publishing port to implement. Consumer code belongs here only for broker protocol mechanics, not for feature-specific handling.

### `user-service/internal/features/<feature>/infrastructure/consumers`

Feature-local `infrastructure/consumers` is optional and should only be created when a feature has a real inbound event consumer. It may contain:

- A consumer handler adapter for one feature.
- Mapping from normalized integration event input to application command/query DTOs.
- Idempotency key extraction or delivery metadata mapping when needed by the application port.
- Error classification that tells the integration layer whether an event should be acked, retried or dead-lettered.
- Fx provider wiring local to the feature module, if the consumer becomes part of the runtime graph.

It must not contain:

- Broker SDK calls, topic subscription loops or low-level ack/nack protocol code.
- Cross-feature orchestration.
- Direct Ent/Redis access when the application use case or feature port should own the behavior.
- HTTP controllers, HTTP DTOs or response envelope logic.
- Shared consumer abstractions for hypothetical future consumers.

The consumer adapter may depend on its feature `application` package and domain models. It should call application use cases or ports rather than reimplementing use case logic.

### Feature `application`

Feature application remains the owner of business orchestration:

- Commands and queries express event-triggered work as transport-neutral inputs.
- Ports express the feature's required external publishing or consuming side effects.
- Application services decide whether an event changes state, emits another intent, is ignored, or fails.

Application code should not import broker SDKs, external event DTOs, integration adapter packages, Ent, Redis clients or Gin.

### Feature `domain/events`

Feature `domain/events` is still only for pure domain event models: names, payload structs and constructors for facts that already happened inside the domain.

It is not an event bus, not an integration schema and not an outbox. It must not contain publisher, subscriber, handler, broker dependency, topic name, outbox state, Ent hook or background worker code.

### `common/runtime/eventbus`

`common/runtime/eventbus` may be introduced only when all of the following are true:

- At least two service modules need the same stable runtime primitive.
- The API has no user-service business semantics.
- The primitive is about runtime concerns such as lifecycle, handler registry, tracing metadata propagation, retry policy contracts or a small publisher interface.
- A concrete broker integration exists or is being introduced in the same implementation change.
- Tests can demonstrate the primitive without requiring a user-service feature.

It should not contain user-service event names, feature ports, broker-specific DTOs that are not truly reusable, or speculative abstractions.

### Outbox Boundary

Outbox is not introduced by this change. A future outbox implementation needs a separate design because it touches transaction semantics and delivery reliability.

Before adding outbox code, the change must define:

- Whether outbox is service-local under `user-service/internal/...` or stable enough for `common/runtime/outbox`.
- The storage model, migration ownership and retention strategy.
- How application use cases write business state and outbox records in one transaction.
- Whether Ent hooks, explicit transaction wrappers or repository methods own the write.
- The delivery worker lifecycle, retry/backoff, idempotency and poison-message strategy.
- How producer adapter errors map to retryable or terminal delivery states.

Until that design exists, do not add outbox tables, Ent hooks, transaction wrappers, workers or publisher loops.

## Producer Flow

A future producer path should look like:

```text
feature application use case
  -> feature-owned publish port
  -> integration/events producer adapter
  -> external broker
```

The feature application decides what business intent should be published. The integration adapter only translates that intent into an external event envelope and broker call.

If reliable delivery is required, producer flow must go through a separately designed outbox path instead of directly adding publish calls to persistence adapters or transactions.

## Consumer Flow

A future consumer path should look like:

```text
external broker
  -> integration/events consumer wrapper
  -> normalized event input
  -> features/<feature>/infrastructure/consumers adapter
  -> feature application command/query
```

`integration/events` owns broker protocol and envelope parsing. The feature consumer adapter owns mapping into feature application inputs. The application use case owns state changes and business decisions.

## Dependency Rules

Recommended dependency boundaries:

| Layer | May depend on | Must not depend on |
|---|---|---|
| `domain/events` | Standard library, same-feature domain models and value objects | Broker SDKs, topic names, publisher/subscriber code, outbox state, Ent, Redis, Gin, application ports |
| `application` | Domain, feature-owned ports, common security/runtime primitives when already allowed | Broker SDKs, integration adapters, external event DTOs, Ent, Redis clients, Gin |
| `infrastructure/consumers` | Feature application, feature domain, normalized event DTOs from `integration/events` if needed | Broker SDK subscription loops, Ent/Redis direct access for business behavior, Gin, cross-feature orchestration |
| `integration/events` | External broker SDK/client, feature application ports, domain value objects, common runtime primitives | Feature use case implementations, Ent, Redis stores, Gin response, outbox persistence |
| `common/runtime/eventbus` | Standard library and cross-service runtime dependencies | User-service feature packages, user-service event names, business DTOs |
| outbox implementation | To be decided by separate design | Any transaction or worker behavior added without explicit design |

## Documentation Updates

Implementation should update:

- `docs/ARCHITECTURE.md`
  - Feature layout table: mention optional `infrastructure/consumers`.
  - Integration section: clarify `integration/events` owns external event system protocol adaptation.
  - Common section: clarify `common/runtime/eventbus` and outbox require concrete cross-service runtime need and separate design.
  - Dependency rules: add event consumer and eventbus/outbox guidance.
  - Current constraints: state there is still no broker, event bus, outbox, publisher, subscriber or worker.
- `AGENTS.md`
  - Repository Shape: mention future `infrastructure/consumers` and eventbus/outbox gates.
  - Repository Rules: state where producer and consumer code belongs, and what is not allowed without a separate change.
- `user-service/internal/integration/events/README.md`
  - Expand current README from generic adapter language into explicit producer/consumer protocol boundary language.

No `openspec/` or `docs/opsx/` artifacts should be created.

## Verification Strategy

For a documentation-only implementation, verify structure and absence of real runtime changes:

```bash
test -f docs/changes/add-event-consumer-producer-boundary/proposal.md
test -f docs/changes/add-event-consumer-producer-boundary/design.md
test -f docs/changes/add-event-consumer-producer-boundary/tasks.md
rg -n "integration/events|infrastructure/consumers|eventbus|outbox" AGENTS.md docs/ARCHITECTURE.md user-service/internal/integration/events/README.md
rg -n "Kafka|RabbitMQ|NATS|Redis Stream|producer|consumer|outbox" go.mod common/go.mod user-service/go.mod user-service/internal common/runtime
```

The final scan may find documentation mentions, but it must not reveal real dependencies, workers, hooks, producers, consumers or outbox implementation.

If any Go package docs or Go code are added, run:

```bash
cd user-service
go test ./...
cd ../common
go test ./...
```

## Risks And Mitigations

### Boundary Overlap

Risk: developers place consumer business logic in `integration/events` because it is already named events.

Mitigation: document that `integration/events` owns broker protocol mechanics, while feature `infrastructure/consumers` maps normalized input into application use cases.

### Premature Common Abstraction

Risk: `common/runtime/eventbus` becomes a speculative framework before multiple services need it.

Mitigation: require concrete multi-service reuse, stable runtime-only semantics and tests before adding common eventbus code.

### Hidden Transaction Changes

Risk: event publication is added inside persistence adapters or Ent hooks before outbox semantics are designed.

Mitigation: explicitly gate outbox, transaction hooks, workers and delivery state behind a separate design.

### Domain Event Confusion

Risk: `domain/events` is treated as integration event schema or broker topic ownership.

Mitigation: keep domain events as pure facts only; external envelope and topic mapping belong in `integration/events`.
