## 1. Common API 与状态机

- [x] 1.1 在 `common/runtime/redispubsub` 定义并注释 `Client`、`Options`、`Message`、`State`、`ErrorCategory`、`Status`、`Subscriber` 与 `ErrStopped`，实现必需依赖及全部 option 的严格校验，保证 constructor 不补默认值、不连接 Redis 且不创建 goroutine。
- [x] 1.2 实现 `created -> starting -> connected/reconnecting -> stopping -> stopped` 单向并发状态机，保证运行期重复 `Start()` 幂等、停止开始后的 `Start()` 返回 `ErrStopped`，并提供无数据竞争的 `Status()` 与唯一 `Messages()` channel。
- [x] 1.3 实现首次和重复 `Stop(ctx)` 的共享完成状态，使调用方超时仅结束当前等待、后台 drain 继续，并在完全停止后只关闭一次消息 channel。

## 2. Redis subscription supervisor

- [x] 2.1 实现单 channel `Subscribe` attempt、带 `SubscribeTimeout` 的显式确认和阻塞 `Receive`，将 go-redis message 投影为 `redispubsub.Message`，并分别记录 subscribe、receive 与 protocol 低基数错误状态。
- [x] 2.2 实现从 `BackoffInitial` 到 `BackoffMax` 的防溢出指数退避及 `[delay/2, delay]` jitter，成功确认后重置退避，确认/接收失败后持续重连且不设置永久终止次数。
- [x] 2.3 实现容量固定为 `BufferSize` 的 context-aware 消息背压，确保缓冲满时不丢弃已读取消息，停止可解除确认、Receive、backoff 与消息交付阻塞。
- [x] 2.4 为每个 attempt 建立单 owner 与 `sync.Once` 关闭 gate，保证 Stop 和失败路径竞争时当前 `PubSub.Close()` 最多一次，且任何路径都不关闭共享 Redis client。

## 3. Common 测试

- [x] 3.1 增加 table-driven 测试覆盖 nil/非法依赖、空白 name/channel、非正 buffer/duration、倒置 backoff、合法 options 及 constructor 无 Redis/goroutine 副作用。
- [x] 3.2 增加状态与重连测试，覆盖 Start 幂等、停止后拒绝 Start、订阅确认失败、Receive 失败、协议类型错误、成功恢复、错误清除、时间戳、重连计数、backoff 上界和 jitter 范围。
- [x] 3.3 增加并发停止测试，覆盖 confirmation、Receive、backoff 和满缓冲交付期间取消、当前 PubSub 恰好关闭一次、共享 client 不关闭，以及 Stop 超时后重复 Stop 等待同一 drain。
- [x] 3.4 使用 `common/testing/containers` Redis Cluster fixture 增加 classic Pub/Sub publish/receive 集成测试，明确不使用 pattern、sharded Pub/Sub、Streams 或可靠投递断言。

## 4. Permission 迁移与装配

- [x] 4.1 从 permission Redis store 的 `policyRedisClient` 删除 `Subscribe`，删除 `policySubscriber`、`policySubscriptionStore` 与 `Store.Subscribe`，新增 `Store.PolicyChannel()` 并保持 publish 和 revision 行为不变。
- [x] 4.2 修改 permission composition，使用共享 cache Redis client、`Store.PolicyChannel()` 和已校验的 `subscribe_timeout`/backoff 配置构造具体 `*redispubsub.Subscriber`，显式传入 `BufferSize: 64`，并让 `WatcherSettings` 只保留 `CheckInterval`。
- [x] 4.3 将 watcher 生产 constructor 改为接收具体 subscriber、内部 constructor 改为接收 feature-local `messageSource`，消费 `Messages()` 并保留消息解码、PostgreSQL revision 查询、reload、缓存失效和周期补偿的串行 RBAC 语义。
- [x] 4.4 删除 watcher 内的 `subscriptionAttempt`、active subscription、subscription supervisor/receive、retry/backoff/jitter 和订阅状态直写逻辑，不提供旧 constructor、deprecated alias、wrapper 或兼容分支。
- [x] 4.5 实现 watcher 组合状态：显式映射 subscriber state/error、连接时间和重连数，自行维护 RBAC root running、reconcile 成功/错误及失败时间，并以 subscription/reconcile 较新值报告 `LastFailureAt`。
- [x] 4.6 将 `Watcher.Start()` 与 `policyWatcherRunner.Start()` 改为返回 `error`，更新 Fx `OnStart` 和全部调用点，确保 watcher 或 dispatcher 启动失败时停止已启动组件，Stop 聚合业务 root 与 subscriber 的同一 drain 结果。

## 5. Permission 行为测试

- [x] 5.1 将 watcher 测试 fake 收敛到最小 `messageSource`，删除已迁入 common 的订阅确认、Receive、重连、退避、PubSub 关闭和缓冲背压故障测试。
- [x] 5.2 保留并完善 RBAC 测试，覆盖 payload 解码、数据库 revision 权威校准、revision-aware reload、定向/全量缓存失效、重复乱序 hint、启动立即校准与周期补偿。
- [x] 5.3 增加 watcher 状态组合测试，覆盖 subscription 与 reconcile 错误独立恢复、protocol 映射、较新 `LastFailureAt`、subscriber 重连期间继续校准和 PostgreSQL 权威健康语义。
- [x] 5.4 更新 lifecycle/provider 测试，覆盖 `Start() error` 传播、重复启动、启动回滚、Stop 超时后重复等待、单一 subscriber/watcher 实例及共享 Redis client 所有权。

## 6. 文档与定向验证

- [x] 6.1 更新 `docs/ARCHITECTURE.md`，登记 `common/runtime/redispubsub` 职责、at-most-once 边界和 permission watcher 对 primitive 的复用关系；更新 `docs/opsx/CAPABILITY_MAP.md` 的 shared primitive 路径与 RBAC 交叉依赖。
- [x] 6.2 对新增和修改的 Go 代码运行格式化并检查导出 symbol 中文文档注释、复杂并发分支必要注释、英文 log message 与稳定 `snake_case` 字段。
- [x] 6.3 运行 `openspec validate add-redis-pubsub-runtime`，确认 proposal、design、两个 capability delta 与 tasks 一致且可解析。
- [x] 6.4 运行 `make common-test`，再在 `common/` module 运行 `go test -race ./runtime/redispubsub`。
- [x] 6.5 在仓库根目录运行 `go test -race ./user-service/internal/features/permission/infrastructure/redis`。
- [x] 6.6 运行 `make user-service-architecture-lint`，确认 common 不含 RBAC 业务语义、permission 分层和文档/规格边界通过门禁。

## 7. 整体验收与收尾

- [x] 7.1 检查工作区 diff，确认不包含非预期 HTTP/OpenAPI、Ent/migration、deployment 或 observability asset 变更，也没有旧订阅实现、兼容 wrapper 或未迁移调用点。
- [x] 7.2 将本次预期的 common、user-service、测试、文档和 `openspec/changes/add-redis-pubsub-runtime` 变更加到暂存区，并用 `git diff --cached` 复核范围。
- [x] 7.3 在预期变更已暂存后运行 `make lint`；失败时修复并重新暂存相关文件，未通过前不得完成本任务。
- [x] 7.4 在预期变更已暂存且 lint 通过后运行 `make verify`，确认完整测试、架构门禁和最终 `git diff --exit-code` 无生成物 drift；未通过前不得完成本任务。
- [x] 7.5 按实际完成情况立即将本文件对应 checkbox 改为 `- [x]`，复核 `openspec status --change "add-redis-pubsub-runtime"` 仍为 apply-ready，并保留 change 等待 `/opsx:archive`。
