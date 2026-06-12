# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只建立事件 producer/consumer 边界。
- [x] 阅读 `user-service/internal/integration/README.md` 和 `user-service/internal/integration/events/README.md`，确认当前 events integration 只有文档占位。
- [x] 检查当前是否已存在 `common/runtime/eventbus`、outbox、feature `infrastructure/consumers`、MQ dependency、worker 或 producer/consumer 实现。
- [x] 确认本次不修改 HTTP API、feature use case、Ent schema、migration、Redis/PostgreSQL transaction 逻辑或 runtime provider graph。

## Boundary Documentation

- [x] 更新 `user-service/internal/integration/events/README.md`，明确它承载外部事件系统 producer/consumer 协议 adapter、envelope/topic 映射、broker 错误归一化和 broker wrapper。
- [x] 在 `user-service/internal/integration/events/README.md` 中明确禁止 feature 业务编排、Ent/Redis store 访问、outbox persistence、Gin response、topic 常量预设和未使用 broker dependency。
- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization，说明 `features/<feature>/infrastructure/consumers` 是按真实需求新增的 feature-local 入站事件 consumer adapter 边界。
- [x] 更新 `docs/ARCHITECTURE.md` Integration 相关章节，说明 `integration/events` 只负责外部事件系统协议适配，不承载 feature-specific handling。
- [x] 更新 `docs/ARCHITECTURE.md` Common Organization 或 Infrastructure 章节，说明 `common/runtime/eventbus` 只有在跨服务稳定 runtime primitive 出现时才可新增。
- [x] 更新 `docs/ARCHITECTURE.md`，说明 outbox 需要另开变更设计 transaction boundary、存储模型、投递 worker、重试、幂等和失败策略。
- [x] 更新 `docs/ARCHITECTURE.md` Dependency Rules，补充 `infrastructure/consumers`、`integration/events`、`common/runtime/eventbus` 和 outbox 的依赖边界。
- [x] 更新 `docs/ARCHITECTURE.md` Current Constraints，确认当前仍没有 broker、eventbus、outbox、publisher、subscriber 或后台投递 worker。
- [x] 更新 `AGENTS.md` Repository Shape，补充事件 producer/consumer、feature consumer adapter、eventbus 和 outbox 的位置规则。
- [x] 更新 `AGENTS.md` Repository Rules，明确不得新增真实 MQ、业务事件发布、outbox 表、worker、Ent hook 或事务逻辑。

## Optional Minimal Placeholders

- [x] 如确需为 `features/<feature>/infrastructure/consumers` 建立占位，只添加 README 或 package doc，并说明没有真实消费者时不得新增业务代码。
- [x] 如确需为 `common/runtime/eventbus` 建立占位，只添加 README 或 package doc，并说明真实实现需要跨服务稳定复用和单独设计。
- [x] 如确需为 outbox 建立占位，只添加 README 或 package doc，并说明真实实现需要单独设计 transaction/storage/worker 语义。
- [x] 不新增空 struct、空 interface、topic 常量、producer API、consumer handler、worker loop 或 broker config。

## Guardrails

- [x] 不接入 Kafka、RabbitMQ、NATS、Redis Stream 或其他 MQ/broker dependency。
- [x] 不实现业务事件发布、订阅、handler、producer、consumer、eventbus registry、outbox dispatcher 或后台 worker。
- [x] 不新增 domain event payload、integration event schema、topic/subject/stream 常量或消息 envelope type，除非同一实现变更有真实消费者并更新设计。
- [x] 不修改 user/auth application command/query、ports、transport/http DTO、controller、route、provider、store adapter 或 transaction semantics。
- [x] 不修改 Ent schema、Ent generated code、Atlas migration、config key、deployment asset 或 go module dependency。
- [x] 不新增横向 `internal/events`、`internal/consumers`、`internal/messaging` 或 `internal/jobs` 业务代码。
- [x] 不新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行结构检查：

```bash
test -f docs/changes/add-event-consumer-producer-boundary/proposal.md
test -f docs/changes/add-event-consumer-producer-boundary/design.md
test -f docs/changes/add-event-consumer-producer-boundary/tasks.md
test -f user-service/internal/integration/events/README.md
```

- [x] 运行文档引用扫描：

```bash
rg -n "integration/events|infrastructure/consumers|eventbus|outbox" AGENTS.md docs/ARCHITECTURE.md user-service/internal/integration/events/README.md
```

- [x] 运行 runtime scope 扫描，确认没有真实事件实现或 dependency：

```bash
rg -n "Kafka|RabbitMQ|NATS|Redis Stream|sarama|amqp|segmentio/kafka|producer|consumer|outbox|eventbus" common/go.mod user-service/go.mod common user-service/internal
```

- [x] 检查 `git diff -- AGENTS.md docs/ARCHITECTURE.md user-service/internal/integration/events/README.md docs/changes/add-event-consumer-producer-boundary`，确认只有边界文档和 proposal artifacts 变更。
- [x] 如新增任何 Go package doc 或 Go code，在对应模块运行 `go test ./...`。

## Review Notes

- [x] 确认文档能回答 producer adapter 应放在 `integration/events`，producer business decision 应放在 feature `application`。
- [x] 确认文档能回答 consumer broker mechanics 应放在 `integration/events`，feature-specific consumer adapter 应放在 feature `infrastructure/consumers`。
- [x] 确认文档能回答 `domain/events` 只承载纯领域事实，不承载 broker schema、topic 或 publisher/subscriber。
- [x] 确认文档能回答 `common/runtime/eventbus` 只有跨服务稳定 runtime primitive 才能新增。
- [x] 确认文档能回答 outbox 必须单独设计 transaction、storage、worker、重试和幂等策略。
- [x] 确认没有把 `integration/events` 写成 feature service、repository、controller 或 background job 的替代品。
