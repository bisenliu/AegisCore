## ADDED Requirements

### Requirement: 通用 Redis Pub/Sub 单 channel 订阅原语

`common/runtime/redispubsub` MUST 提供业务中立的 Redis classic Pub/Sub 单 channel subscriber，负责显式订阅确认、阻塞接收、有界消息缓冲、context 背压、带抖动的有界指数退避重连、单向生命周期和结构化状态。该 primitive MUST 保持 Redis Pub/Sub 的 at-most-once 通知语义，MUST NOT 承担 Publish 业务封装、消息 envelope、revision、outbox、幂等、数据库校准、Casbin reload、缓存失效、模式订阅、sharded Pub/Sub、Redis Streams 或可靠投递。

#### Scenario: 严格 options 与无副作用构造

- **WHEN** 调用方构造 subscriber
- **THEN** `Options.Name` 与 `Options.Channel` MUST 为非空非空白字符串，`BufferSize`、`SubscribeTimeout`、`BackoffInitial` 与 `BackoffMax` MUST 为正数，且 `BackoffMax` MUST 大于或等于 `BackoffInitial`
- **AND** 任一 option 非法或必需依赖缺失时 `NewSubscriber` MUST 返回 error，primitive MUST NOT 补默认值、静默归一化非法值或进入部分可用状态
- **AND** constructor MUST NOT 创建 goroutine、调用 Redis、建立订阅或关闭共享 client，初始状态 MUST 为 `created` 且 `Running` MUST 为 false

#### Scenario: 单向启动状态

- **WHEN** created subscriber 首次调用 `Start()`
- **THEN** subscriber MUST 原子进入 `starting`、创建唯一 root lifecycle 并异步建立订阅
- **WHEN** subscriber 已处于 starting、connected 或 reconnecting 状态并再次调用 `Start()`
- **THEN** `Start()` MUST 幂等返回 nil 且 MUST NOT 创建额外 goroutine或订阅
- **WHEN** subscriber 已进入 stopping 或 stopped 状态并调用 `Start()`
- **THEN** `Start()` MUST 返回稳定的 `ErrStopped`，MUST NOT 恢复或创建新的生命周期

#### Scenario: 显式确认与成功接收

- **WHEN** subscriber 为 `Options.Channel` 创建一次 classic Pub/Sub attempt
- **THEN** subscriber MUST 使用不超过 `SubscribeTimeout` 的 context 阻塞等待 go-redis 订阅确认，确认前 MUST NOT 将状态报告为 connected
- **WHEN** 收到合法订阅确认
- **THEN** subscriber MUST 将状态置为 `connected`、记录 `LastConnectedAt`、将当前 `ErrorCategory` 清为 `none` 并把下一次退避重置为 `BackoffInitial`
- **WHEN** 已确认订阅收到合法 go-redis message
- **THEN** subscriber MUST 将 `Channel`、`Pattern` 和 `Payload` 投影为公开 `Message` 并写入唯一的 `Messages()` channel

#### Scenario: 故障分类与有界重连

- **WHEN** 等待订阅确认的 Receive 失败或超时
- **THEN** subscriber MUST 记录 `subscribe_failed`、`LastFailureAt` 和重连次数，关闭当前 attempt 并进入 `reconnecting`
- **WHEN** 已确认订阅的 Receive 返回非取消错误
- **THEN** subscriber MUST 记录 `receive_failed`、`LastFailureAt` 和重连次数，关闭当前 attempt 并进入 `reconnecting`
- **WHEN** 确认或接收阶段收到不受支持的协议类型
- **THEN** subscriber MUST 记录 `protocol_failed`、`LastFailureAt` 和重连次数，关闭当前 attempt 并进入 `reconnecting`
- **AND** 重连 MUST 使用从 `BackoffInitial` 指数增长且不超过 `BackoffMax` 的基准延迟，每次实际等待 MUST 位于对应基准延迟的 `[1/2, 1]` 闭区间，且 MUST NOT 设置永久终止次数

#### Scenario: 有界缓冲与 context 背压

- **WHEN** `Messages()` 缓冲尚有容量
- **THEN** subscriber MUST 按接收顺序交付消息且缓冲容量 MUST 等于显式 `BufferSize`
- **WHEN** 缓冲已满且 subscriber 已从 Redis 读取下一条消息
- **THEN** 接收 goroutine MUST 阻塞等待消费者腾出容量，MUST NOT 主动丢弃该消息、扩展为无界缓冲或启动绕过背压的额外交付 goroutine
- **WHEN** subscriber 在消息交付阻塞期间被停止
- **THEN** root context 取消 MUST 解除阻塞并允许 lifecycle drain 完成

#### Scenario: 单 owner 停止与资源所有权

- **WHEN** subscriber 首次调用 `Stop(ctx)`，包括尚未 Start 的 created 状态
- **THEN** subscriber MUST 单向进入 `stopping`、取消 root context、触发活动 PubSub 关闭并等待唯一完成状态
- **AND** 订阅确认、Receive、重连 backoff 或缓冲交付中的阻塞 MUST 能被停止解除，取消后 MUST NOT 创建新 attempt
- **WHEN** Stop 与 attempt 失败路径并发关闭同一 PubSub
- **THEN** 每个 attempt MUST 通过单 owner 与恰好一个 close-once gate 保证 `PubSub.Close()` 最多执行一次，subscriber MUST NOT 关闭共享 Redis client
- **WHEN** lifecycle 完全 drain
- **THEN** subscriber MUST 将状态置为 `stopped`、将 `Running` 置为 false，并且只关闭一次 `Messages()` channel

#### Scenario: Stop 超时与重复等待

- **WHEN** `Stop(ctx)` 的调用方 context 在后台 lifecycle drain 完成前结束
- **THEN** 本次 Stop MUST 返回 context error，但后台 drain MUST 继续，完成状态、消息 channel 和 stopping 状态 MUST NOT 被替换或重置
- **WHEN** 调用方随后再次调用 `Stop(ctx)`
- **THEN** 后续调用 MUST 等待同一个完成状态，并在该 drain 已完成时幂等返回 nil

#### Scenario: Redis Cluster classic Pub/Sub 边界

- **WHEN** subscriber 使用仓库 Redis Cluster fixture 订阅 channel 且另一个 client 通过 classic `Publish` 发送消息
- **THEN** subscriber MUST 能完成订阅确认并接收对应消息
- **AND** 该验证 MUST NOT 使用 `PSubscribe`、`SSubscribe`、Redis Streams 或宣称断线、重连和背压期间的消息可恢复
