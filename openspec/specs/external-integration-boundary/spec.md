# 外部集成边界规格

## 需求

### 需求：集成包用途
`user-service/internal/integration/http|grpc|events` 只能用于真实外部系统协议适配器和防腐层。

#### 场景：新增外部 HTTP client
Given 功能需要调用真实外部 HTTP 系统
When 定义集成边界
Then 功能 application 必须拥有最小端口，`integration/http` 可以实现协议适配器。

### 需求：禁止推测性集成
仓库不得在没有单独设计的情况下新增推测性的 order/payment client、broker wrapper、eventbus、outbox、producer、subscriber、consumer handler、dispatcher 或 event worker。

#### 场景：新增 MQ 依赖
Given 有人提议 Kafka、RabbitMQ、NATS、Redis Stream、eventbus 或 outbox
When 没有单独批准的设计
Then 不得新增依赖、表、Ent hook、transaction wrapper、dispatcher 或 worker。

### 需求：gRPC 与事件边界分离
`integration/grpc` 表示出站外部 gRPC client adapter，不表示本服务入站 gRPC transport。`integration/events` 只拥有 broker 协议机制，不拥有功能特定消费者处理器或业务编排。

#### 场景：新增入站 gRPC API
Given user-service 暴露真实入站 gRPC API
When 放置处理器代码
Then 必须放在所属功能的 `transport/grpc`，不得放在 `internal/integration/grpc`。

#### 场景：新增事件消费者
Given 已设计真实外部 broker consumer
When 划分职责
Then broker subscription、ack 和协议机制必须放在 `integration/events`，功能特定映射到 application command/query 的适配器必须放在所属功能的 `infrastructure/consumers`。
