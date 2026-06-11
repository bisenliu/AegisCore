# Events Integration

`integration/events` 用于用户服务访问外部事件系统的 producer/consumer adapter。

可以放置：

- 事件 envelope 与外部 topic、subject 或 stream 的映射。
- broker-specific producer/consumer wrapper。
- 投递结果、重试、幂等和反序列化错误归一化。
- 对 feature app port 或应用事件处理入口的适配。

禁止放置：

- 尚无真实需求的 Kafka、RabbitMQ、NATS 或其他 broker 依赖。
- feature app service 内部业务编排。
- 本服务 HTTP controller、Ent adapter 或 Redis store。

当前没有真实外部事件系统调用，因此本目录只保留 README 占位。
