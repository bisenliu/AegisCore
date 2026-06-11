# Events Integration

`integration/events` 是用户服务访问外部事件系统的协议 adapter 边界。它只负责外部 broker、topic/subject/stream、事件 envelope、序列化、投递结果和 broker 错误语义适配，不负责 feature 业务编排。

## 可以放置

- 外部事件 envelope、headers、trace metadata、partition key 和序列化映射。
- 外部 topic、subject、stream 或 routing-key 到内部意图的映射。
- broker-specific producer/consumer wrapper。
- ack、nack、retry、dead-letter、投递结果和反序列化错误归一化。
- 对 feature application 拥有的最小 publish port 的实现。
- 面向 feature-local consumer adapter 的归一化入站事件输入。

## 禁止放置

- 尚无真实需求的 Kafka、RabbitMQ、NATS、Redis Stream 或其他 broker 依赖。
- 预设 topic 常量、producer API、consumer handler、worker loop 或 broker config。
- feature application service 内部业务编排、command/query 实现或跨 feature orchestration。
- Ent、SQL、Redis store、本服务持久化 adapter 或 outbox persistence。
- Gin controller、HTTP route、HTTP request/response DTO 或 response envelope 输出。
- 为 adapter 自身方便而扩张的通用大接口。

## 与 Feature Consumer 的关系

未来如某个 feature 需要消费外部事件，broker 协议 mechanics 留在 `integration/events`；feature-specific 映射和 handler adapter 放在 `user-service/internal/features/<feature>/infrastructure/consumers`，并调用该 feature 的 application command/query 或 port。

## 与 Outbox 的关系

当前没有 outbox。可靠投递需要另开变更设计 transaction boundary、存储模型、投递 worker、重试、幂等和失败策略；不要在本目录中顺手新增 outbox 表、Ent hook、transaction wrapper 或后台投递循环。

当前没有真实外部事件系统调用，因此本目录只保留 README 占位。
