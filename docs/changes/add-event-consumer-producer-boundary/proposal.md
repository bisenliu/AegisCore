# Add event consumer producer boundary

## What

明确用户服务事件 producer/consumer 的代码归属边界，为未来 `events`、feature-local `consumers` 和 outbox 能力做准备。本变更只建立架构规则与最小边界文档，不接入真实 MQ，不实现业务事件发布，也不修改事务逻辑。

目标边界：

```text
user-service/internal/integration/events/
  README.md        # 外部事件系统协议 adapter 边界

user-service/internal/features/<feature>/infrastructure/consumers/
  README.md        # feature 消费入站事件后的 adapter 边界，按真实消费需求新增

common/runtime/eventbus/
  README.md        # 未来跨服务稳定事件总线 runtime primitive 的准入边界，按真实复用需求新增

common/runtime/outbox/
  README.md        # 未来跨服务稳定 outbox 基础能力的准入边界，按真实复用需求新增
```

`integration/events` 负责外部 broker、topic、envelope、producer/consumer wrapper、序列化和 broker 错误语义适配。Feature-local `infrastructure/consumers` 负责把已归一化的外部事件输入转换为该 feature 的 application command/query 或 port 调用。未来 `common/runtime/eventbus` 与 outbox 只能在有真实跨服务复用和稳定接口时引入。

## Why

当前仓库已经声明 `internal/integration/events` 是外部事件系统防腐层边界，但尚未明确 producer、consumer、feature handler、eventbus runtime 和 outbox 的相互关系。后续一旦引入事件驱动能力，代码容易漂移到几类不清晰的位置：

- 在 `integration/events` 中实现 feature 业务编排或直接调用 Ent/Redis store。
- 在 feature `application` 中直接依赖 Kafka、RabbitMQ、NATS 等 broker SDK 和外部 event DTO。
- 在 `common` 中过早抽象事件总线或 outbox，形成没有真实消费者的跨服务公共代码。
- 在 transaction 或 persistence adapter 中顺手加入事件发布，导致事务语义和投递语义耦合不清。

本变更提前固定边界语言，让事件相关代码未来能按职责落位：外部协议适配归 integration，feature 消费适配归 feature infrastructure，业务编排归 feature application，跨服务基础能力归 common 且需要更高准入门槛。

## Scope

包括：

- 明确 `user-service/internal/integration/events` 的 producer/consumer adapter 职责和禁止事项。
- 明确未来 `user-service/internal/features/<feature>/infrastructure/consumers` 的准入条件、职责和依赖规则。
- 明确未来 `common/runtime/eventbus` 的准入条件：仅承载跨服务稳定、无业务语义的事件总线 runtime primitive。
- 明确未来 outbox 的准入条件：需要单独设计 transaction boundary、存储模型、投递 worker、幂等和重试策略。
- 更新 `docs/ARCHITECTURE.md` 和 `AGENTS.md`，说明事件代码应放在哪里。
- 视需要补充 README 或 package doc 占位，但不得新增未使用 Go declaration。

不包括：

- 不接入 Kafka、RabbitMQ、NATS、Redis Stream 或其他真实 MQ/broker。
- 不实现业务事件发布、订阅、handler、后台 worker 或投递循环。
- 不新增 domain event payload、integration event schema、topic 常量或 producer API。
- 不修改 user/auth application use case、store port、Ent schema、migration 或事务逻辑。
- 不新增 outbox 表、Ent hook、transaction wrapper、CDC 或 exactly-once 语义。
- 不新增横向 `internal/events`、`internal/consumers`、`internal/messaging` 或 `internal/jobs` 业务代码。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `docs/ARCHITECTURE.md` 说明事件 producer/consumer、feature consumer adapter、eventbus 和 outbox 的代码归属。
- `AGENTS.md` 同步说明事件边界规则，便于后续代理遵循。
- `user-service/internal/integration/events/README.md` 明确它只承载外部事件系统协议适配，不承载 feature 业务编排。
- 如新增 feature-local `infrastructure/consumers` 占位，只能是 README 或 package doc，并且必须说明没有真实消费者时不创建业务代码。
- 如新增 `common/runtime/eventbus` 或 outbox 占位，只能是 README 或 package doc，并且必须说明真实实现需要单独变更。
- 工作树中不存在真实 MQ dependency、producer/consumer implementation、outbox table、worker、Ent hook 或事务逻辑变更。
- 文档能清楚回答：事件代码应该放在 `integration/events`、feature `infrastructure/consumers`、feature `application`、`domain/events`、`common/runtime/eventbus` 还是 outbox 边界。
