## Context

`user-service/internal/features/permission/infrastructure/redis/watcher.go` 当前同时实现两种不同层级的职责：一是 Redis Pub/Sub 的订阅确认、阻塞接收、缓冲、退避重连、attempt 关闭与订阅状态；二是 RBAC 消息解码、PostgreSQL latest policy revision 校准、Casbin reload、用户角色缓存失效和健康状态组合。前一组行为不含 RBAC 语义，适合作为 `common/runtime` primitive；后一组行为必须继续留在 permission feature。

本变更跨越 `common` 与 `user-service`，并修改内部生命周期 API。现有 Redis client 是由 datastore/Fx 管理的共享 `redis.UniversalClient`，subscriber 只能关闭自己创建的 `*redis.PubSub`，不能关闭共享 client。Redis classic Pub/Sub 仍是可丢失的 at-most-once 通知，PostgreSQL revision 和 watcher 周期补偿仍负责 RBAC 最终收敛。

受影响路径包括新增的 `common/runtime/redispubsub/`，以及 `user-service/internal/features/permission/infrastructure/redis/`、`user-service/internal/features/permission/fx_sync.go`、`fx_lifecycle.go`、相关配置接线与测试。`docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 和两个 OpenSpec capability 必须同步；`internal/shared`、`internal/integration`、deployments 和观测资产不承载该 primitive。

## Goals / Non-Goals

**Goals:**

- 提供业务中立、严格配置、可复用的 Redis 单 channel subscription primitive。
- 明确定义订阅确认、消息接收、协议校验、有界缓冲、context 背压、抖动退避重连、状态和资源关闭行为。
- 使用单 owner 与每 attempt 的 `sync.Once` 保证当前 PubSub 最多关闭一次，并使停止超时后的重复 `Stop` 等待同一个后台 drain。
- 一次性把 permission watcher 的通用订阅职责迁出，并保持 PostgreSQL 权威校准、Casbin fail-closed、消息副作用和周期补偿行为不变。
- 让 common 测试覆盖通用故障与并发生命周期，让 permission 测试只覆盖 RBAC 语义。

**Non-Goals:**

- 不提供 Publish 业务封装、JSON envelope、revision、outbox、幂等键、PostgreSQL 校准、Casbin reload 或缓存失效。
- 不支持 `PSubscribe`、`SSubscribe`、Redis Streams、多 channel 动态管理或可靠投递保证。
- 不引入 eventbus、external integration、deprecated alias、兼容 wrapper 或旧构造器。
- 不修改 HTTP API、数据库 schema/migration、OpenAPI 生成物、部署清单、Prometheus/Grafana 资产或安全授权模型。

## Decisions

### Decision: 在 `common/runtime/redispubsub` 暴露最小单 channel API

包提供用户给定的 `Client`、`Options`、`Message`、`State`、`ErrorCategory`、`Status` 和 `Subscriber` API。`Client` 只声明 `Subscribe(ctx, channels...) *redis.PubSub`；`Subscriber` 只暴露 `Start() error`、`Messages() <-chan Message`、`Status() Status` 与 `Stop(ctx) error`。虽然 go-redis 的方法接受可变 channel，primitive 每个实例只使用 `Options.Channel` 创建一个 classic Pub/Sub 订阅。

`NewSubscriber` 对依赖与全部 option 做严格校验：`Name`、`Channel` 必须为非空非空白字符串，`BufferSize`、`SubscribeTimeout`、`BackoffInitial`、`BackoffMax` 必须为正数，且 `BackoffMax >= BackoffInitial`。constructor 不修剪并回写配置、不补默认值、不连接 Redis、不创建 goroutine；服务默认值继续由 `user-service/internal/config` 提供，composition 显式传入 `BufferSize: 64`。导出 symbol 使用中文文档注释，日志消息使用英文且字段为稳定 `snake_case`。

备选方案是在 permission 包中抽 helper，或直接暴露 `redis.PubSub.Channel()`。前者不能形成跨服务 primitive，后者隐藏订阅确认并使用 go-redis 自己的 channel 行为，无法完整表达本变更的协议错误、背压和关闭契约，因此不采用。

### Decision: 单向状态机与一个共享完成状态

生命周期状态固定为 `created -> starting -> connected <-> reconnecting -> stopping -> stopped`。constructor 返回 `created` 且无副作用；首次 `Start` 原子进入 `starting` 并创建唯一 root context 和 supervisor，已处于 starting/connected/reconnecting 时 `Start` 幂等返回 nil。停止一旦开始即不可逆，在 stopping 或 stopped 上调用 `Start` 返回稳定的 `ErrStopped`。

首次 `Stop` 包括从 created 状态停止，都会进入 stopping、取消 root context、触发当前 PubSub 关闭并等待同一个完成 channel。调用方 context 超时只结束本次等待并返回 context 错误，不替换完成 channel、不重置状态，也不取消后台 drain；后续 `Stop` 继续等待同一完成状态。drain 完成后由唯一 owner 置为 stopped，并且只关闭一次 `Messages()` channel。`Status.Running` 在后台 root/drain 尚未完成时为 true，在 created 与 stopped 时为 false。

备选方案是让超时的 `Stop` 放弃资源或允许 stopped 后重启。前者会泄漏 PubSub/goroutine，后者需要重新创建输出 channel 并引入双向状态与代际竞态，因此不采用。

### Decision: supervisor 显式确认、接收、背压与重连

每次 attempt 调用一次 `Client.Subscribe`，以 `SubscribeTimeout` 派生 context 阻塞等待首个订阅确认。确认 Receive 失败归类为 `subscribe_failed`；确认或运行期收到不支持的协议类型归类为 `protocol_failed`；确认后的 Receive 失败归类为 `receive_failed`。故障会记录 `LastFailureAt`、推进 `Reconnects`、将状态置为 reconnecting，并关闭当前 attempt 后重试；成功确认会记录 `LastConnectedAt`、清除当前错误并把退避重置为初始值。

重试使用以 `BackoffInitial` 起步、封顶 `BackoffMax` 的指数退避，并在每次实际等待时施加 `[delay/2, delay]` 的非安全随机 jitter。订阅确认、Receive、退避 timer 和向有界 `Messages()` 缓冲写入都受 root context 控制。缓冲满时接收 goroutine阻塞且不丢弃已经从 Redis 读取的消息；取消必须通过同一个 select 解除阻塞。该背压只能保护进程内有界内存，不能把 Redis Pub/Sub 提升为可靠队列。

每个 attempt 由单一 supervisor 持有，并用 `sync.Once` 包装 `PubSub.Close()`；Stop 与失败路径即使竞争，也最多关闭一次当前 PubSub。subscriber 从不调用共享 `Client.Close()`。

备选方案是满缓冲时 drop、启用无界队列或使用 Redis Streams。drop 会破坏“已读取消息不主动丢弃”的契约，无界队列没有内存边界，Streams 则改变协议与可靠性模型，均不采用。

### Decision: permission watcher 通过最小消息源组合状态

permission 包定义 feature-local `messageSource`：

```go
type messageSource interface {
	Start() error
	Stop(context.Context) error
	Messages() <-chan redispubsub.Message
	Status() redispubsub.Status
}
```

生产 `WatcherParams` 接收具体 `*redispubsub.Subscriber`，内部 `newWatcher` 接收 `messageSource` 以便 RBAC 行为测试注入最小 fake。`WatcherSettings` 只保留 `CheckInterval`；subscribe timeout 与 backoff 从已校验的 user-service 配置直接传给 `redispubsub.Options`。permission provider 使用共享 Redis client、`Store.PolicyChannel()` 和 `BufferSize: 64` 构造 subscriber，再构造 watcher；`Store` 删除 `Subscribe`，其 client 接口删除 `Subscribe`，但 publish/revision 行为保持不变。

`Watcher.Start() error` 先启动 message source，成功后启动唯一的 RBAC 消费/周期校准 root；已运行时幂等，停止后不再重启。Fx `OnStart` 与 `policyWatcherRunner` 同步采用错误返回；如果 watcher 或后续 dispatcher 启动失败，lifecycle 回滚停止已经启动的组件。watcher 停止会取消业务 root、停止 source 并等待同一业务 drain，聚合必要错误而不关闭共享 Redis client。

`Watcher.Status()` 每次读取 subscriber 快照并显式映射 state、error、`LastConnectedAt` 与 `Reconnects` 到 `PolicyWatcherStatusSnapshot`。watcher 自己只维护 RBAC root running、`LastReconcileSuccessAt`、reconcile error category 和最近 reconcile failure time；最终 `LastFailureAt` 取 subscription 与 reconcile 两者较新值。`created`/`stopping`/`stopped` 映射到 RBAC stopped，其他 state 按同名阶段映射；protocol error 保留独立低基数映射。subscription 恢复不得清除 reconcile 错误，reconcile 恢复也不得清除 subscription 错误。

备选方案是让 watcher 继续复制 subscriber 状态，或让 common 接收 RBAC callback。复制状态会重新引入并发双写，业务 callback 会污染 `common` 边界，因此不采用。

### Decision: 测试按所有权边界迁移

`common/runtime/redispubsub` 的单元与集成测试覆盖严格校验、constructor 无副作用、真实 Redis publish/receive、确认/Receive/协议错误重连、退避上界与 jitter 范围，以及确认、Receive、backoff、缓冲阻塞期间停止。fake client/PubSub 边界用于精确断言每 attempt 恰好关闭一次、共享 client 不关闭、Stop 超时后重复 Stop 复用同一 drain；现有 `common/testing/containers` Redis Cluster fixture 验证 classic Pub/Sub。

permission watcher 测试删除通用订阅 fault injection，只保留 payload 解码、数据库 revision 权威校准、reload、缓存失效、周期补偿、启动错误传播与组合状态。并发代码使用 race detector 验证。

## Risks / Trade-offs

- [Risk] 有界消息缓冲满时会暂停 Redis socket 消费，且 Redis Pub/Sub 不会为断线或慢消费者补发消息。 -> Mitigation：明确 at-most-once 语义，保留 PostgreSQL revision 周期补偿作为 RBAC 权威收敛路径，并固定小而明确的业务缓冲容量。
- [Risk] Stop、Receive 失败与订阅确认超时可能同时关闭 attempt。 -> Mitigation：单 supervisor owner、attempt-local `sync.Once`、单向状态机和 race 测试共同约束关闭路径。
- [Risk] Stop 调用方超时后后台 drain 尚未完成，短时间内状态仍为 stopping。 -> Mitigation：保留同一 done 状态供后续 Stop 等待，只有真正 drain 完成才关闭 Messages 并发布 stopped。
- [Risk] 通用状态与 RBAC 状态存在类型差异，映射遗漏会使 health 误判。 -> Mitigation：使用显式穷举映射和表驱动测试，并以两条路径较新的失败时间组合 `LastFailureAt`。
- [Trade-off] primitive 只支持单 channel classic Pub/Sub，调用方若要多个 channel 需构造多个 subscriber。该限制换取更小 API、更明确的 channel 身份和所有权。

## Migration Plan

1. 先实现并单独验证 `common/runtime/redispubsub` API、状态机、并发关闭、退避和 Redis Cluster classic Pub/Sub 集成测试。
2. 在 permission composition 中构造 subscriber，并将 watcher 改为消费 `messageSource`；同步迁移 `Start() error` 的 runner、Fx hook 和全部测试调用点。
3. 删除 permission store/watcher 的旧订阅接口、supervisor、attempt、退避与状态直写实现，不保留兼容层；运行 permission 定向 race 测试确认 RBAC 行为不变。
4. 更新架构文档、能力地图和 OpenSpec artifacts，依次运行 `openspec validate add-redis-pubsub-runtime`、`make common-test`、common package race 测试、permission package race 测试和 architecture lint。
5. 将本次预期变更暂存后运行 `make lint` 与 `make verify`。本变更不需要数据库、OpenAPI、部署或观测资产生成步骤，可随普通 user-service 版本滚动发布。

回滚时整体恢复旧 permission watcher 订阅实现及原 lifecycle 签名，并删除未被其他调用方使用的 common package；不涉及数据回滚、migration 或协议兼容窗口。由于不提供双实现切换，不能只回滚 composition 而保留不匹配的 watcher API。

## Open Questions

无。范围、公开 API、状态机、配置归属、测试迁移和 RBAC 语义均已确定。
