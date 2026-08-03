## Context

前置 change `add-rbac-policy-revision-outbox` 已让在线角色、角色权限和用户角色 mutation 在同一 PostgreSQL transaction 中写入单调 `policy_revision` 与 pending `rbac_policy_outbox_events`，并移除 Redis `INCR` 的权威 revision 语义。现有 outbox schema 已包含 event/revision、kind/reason、相关对象 ID、status、attempt、next attempt、last error、幂等键和 delivered time，但只允许 `pending|delivered`，也没有 claim owner 或 lease 字段；因此还不能由多个副本安全地在 transaction 外执行 Redis publish。

本 change 横跨 permission application、permission PostgreSQL/Redis infrastructure、user-service 私有配置、Fx lifecycle、健康路由和 feature metrics。dispatcher 消费 role 写侧创建的持久事件，但事件模型仍是 user-service RBAC 业务模型，不下沉到 `common/`，也不引入通用 MQ、eventbus、outbox framework 或 `internal/integration/events`。HTTP API、OpenAPI 和 Casbin policy 数据结构不变。

## Goals / Non-Goals

**Goals:**

- 从 PostgreSQL 自动恢复并投递 due pending/failed outbox event，使进程崩溃、Redis 暂时不可用和 publish 失败均不需要新的 RBAC mutation 才能恢复。
- 建立支持多副本的 claim lease、publish、条件 ack、失败记录与有界指数退避协议，确保同一有效 lease 只有一个 owner，并让 lease 过期事件可重领。
- 让 dispatcher 成为 Redis revision 通知的可靠发布路径，Redis 只保存已知最大数据库 revision并承载可重复、可乱序的加速消息。
- 通过稳定的新消息 envelope 携带 revision、event ID、kind、reason 和相关对象 ID，使 watcher 能执行幂等的全局 reload 或定向用户角色缓存失效。
- 显式接入 Fx lifecycle、私有配置、低基数 metrics、结构化日志和只读 health/readiness 状态。

**Non-Goals:**

- 不改变 role mutation、policy revision 与 outbox event 的同 transaction 写入模型，也不把 Redis publish 纳入数据库 transaction。
- 不实现按某个历史 revision 构造 Casbin policy 的 revision-aware reload；reload 始终读取当前 PostgreSQL 权威投影。
- 不改造用户角色 cache generation，不引入通用 consumer、broker、dead-letter queue、永久失败终态或人工重放 API。
- 不保留旧 Redis `INCR` counter、旧 Pub/Sub payload 或双协议 fallback，也不改变 HTTP DTO、路由或 OpenAPI 生成物。

## Decisions

### 1. permission feature 拥有 dispatcher application port 与 adapter

dispatcher 位于 `user-service/internal/features/permission/application/`，定义 transport-neutral 的 `OutboxStore`、`PolicyRevisionPublisher`、clock/backoff settings 和只读 status contract；PostgreSQL claim/ack adapter 位于 permission infrastructure，Redis publisher 继续位于 permission Redis infrastructure。虽然 outbox 由 role mutation 创建，但其消费副作用是 permission policy sync，消费侧拥有最小 port 可避免 role application 依赖 Redis 或 permission infrastructure。

备选方案是在 role infrastructure 中实现 dispatcher，或抽取到 `common`/通用 eventbus。前者会让 role feature 拥有授权投影传播，后者会把仅有一个消费者的 RBAC payload、状态机和 key schema误抽象为平台能力，均不采用。dispatcher 可直接使用共享 Ent client，但 named `primary_db`、`cache_redis`、Fx 和 lifecycle metadata 只留在 permission composition。

### 2. 用持久 claim token 与 lease 实现数据库仲裁

扩展 outbox schema：status 允许 `pending|processing|failed|delivered`，新增 nullable `claim_token` 与 `claimed_until`。每轮 claim 在短 PostgreSQL transaction 中按 `revision ASC` 选择到期的 pending/failed event或 lease 已过期的 processing event，使用行锁与 `FOR UPDATE SKIP LOCKED` 避免副本互相阻塞，并原子更新为 processing、写入本轮随机 claim token 与 `claimed_until=now+claim_timeout`。batch size 只限制单轮 claim 数，不改变 revision 权威性。

publish 在 claim transaction 提交后执行，避免持有数据库锁跨越 Redis I/O。ack 和 failure update 必须同时匹配 event ID、`status=processing` 与 claim token；旧 owner 在 lease 已过期且被重新 claim 后不能 ack 或覆盖新 owner 结果。ack 设置 delivered、delivered_at 并清除 claim/error；失败设置 failed、递增 attempt、记录截断后的安全错误摘要、计算 next_attempt_at 并清除 claim。attempt 在实际 publish 失败时递增，而不是在 claim 时递增；进程在 publish 后、ack 前崩溃会产生至少一次重复投递，但不会丢事件。

备选方案是 transaction 内 publish，无法获得 PostgreSQL 与 Redis 的原子性且会长期持锁；只用 status compare-and-swap 无法回收崩溃 claimant；使用进程内 mutex 或 Redis lock 不能跨副本可靠仲裁。数据库 lease 是恢复事实所在系统内最小且可验证的方案。

### 3. 使用有界指数退避并持续保留失败事件

配置包括正数 `poll_interval`、`batch_size`、`claim_timeout`、`retry_backoff.initial` 和 `retry_backoff.max`，且 max 不小于 initial。第 N 次 publish 失败后使用 `min(initial*2^(N-1), max)` 计算下一次尝试时间；实现必须防止 duration/指数溢出。当前不增加 jitter，以便配置、测试与运维预测保持简单；多副本通过数据库 claim 竞争自然分散轮询。

failed 不是终态，不设最大 attempt，也不删除事件。成功 ack 是唯一 delivered 转换。无效持久 payload 属于不可发布错误，但仍按相同退避保留并通过 metrics/log/health 暴露，避免静默吞掉审计事实；修复数据或代码后可自动恢复。

### 4. dispatcher 是唯一异步 Redis 发布者，本地同步保持即时

在线 command 在数据库 commit 后仍立即执行本实例需要的 policy reload 或用户角色缓存失效，但不再把 Redis publish 成功作为 mutation 同步成功条件；跨副本 Redis 发布由 outbox dispatcher 完成。这样 Redis 故障不会要求调用方重做 mutation，也不会出现 direct publisher 已成功但 pending event 无法判断是否 ack 的双写竞态。

数据库 outbox 只保证至少一次投递。Redis publisher 先以原子 max 语义更新 revision cache，再发布消息；重复执行不得降低缓存 revision。任一步失败都不能 ack，后续可完整重试。因为 max 更新成功而 publish 失败时 watcher 的周期检查仍可发现 revision，而 outbox 仍会重试消息，两个路径均只作为加速/补偿而非权威事实。

备选方案是保留同步 direct publish 并由 dispatcher 再发布，虽然也可依赖幂等消费，但会制造稳定重复流量和模糊 ack 所代表的投递路径，因此不采用。不要求 Redis 更新与 Pub/Sub publish 原子化，因为 Redis 不是可靠队列，可靠性由 PostgreSQL event 保证。

### 5. 新消息 envelope 支持重复与乱序消费

JSON payload 使用显式版本化的新结构，至少包含 `schema_version`、`event_id`、`idempotency_key`、`policy_revision`、`kind`、`reason`、可选 `user_id`/`role_id`/`permission_id` 和 publisher instance ID。UUID 使用规范字符串；缺失必需字段、未知 schema version/kind 或非法 UUID 必须拒绝并记录安全错误，不回退解析旧 payload。

多 dispatcher、publish 后 ack 前崩溃和 Redis Pub/Sub 本身都允许重复或乱序。watcher 对 `policy_changed` 执行读取当前 PostgreSQL 投影的全量 reload，对 `user_role_changed` 始终对消息中的 user ID 执行幂等 cache invalidation；不得仅因 revision 小于或等于本地已知最大值而跳过该消息要求的副作用。完成副作用后 tracker 只以 max 语义推进。这样较新但无关的定向事件不会使较旧用户的必要失效被跳过，也不等同于按历史 revision reload。

### 6. lifecycle、status 与观测保持只读和低基数

`NewDispatcher` 只构造对象，不启动 goroutine 或访问外部资源。permission runtime 聚合对象暴露同一 dispatcher 实例的 runner/status 视图；Fx hook 在数据库/Redis resource 可用且 resolver/cache 已启动后启动 dispatcher，并在停止阶段先取消循环、等待 in-flight publish 在 stop context 内结束，再继续关闭其他 RBAC 资源。Start/Stop 幂等，dispatcher 不关闭共享 Ent 或 Redis client。

status 至少包含 running、最近成功 dispatch 时间、最近错误类别、最老未完成 event 的 age/lag 与 due count 快照。health/readiness 只调用 status/store 的只读查询，不 claim、不 publish、不修改 retry 时间；未启动、循环意外退出或无法读取 outbox 状态时 readiness 失败。单次 publish 失败和可重试 backlog 必须可见，但不会终止循环；既有共享 Redis readiness 语义不在本 change 中改写。

feature metrics 记录 claim 数、publish/ack/failure 结果、retry 次数、due backlog、最老未完成 event age 和 loop 状态；label 只允许固定 result/reason/kind 枚举，不包含 event/revision/user/role/permission ID、错误文本、Redis key 或 SQL。日志使用英文 message 与稳定 `snake_case` 字段，可记录 policy revision、attempt、kind/reason 和错误类别，但不得记录完整 payload、SQL、Redis key 或原始底层错误作为公共 health 内容。

## Risks / Trade-offs

- [publish 成功但 ack 前崩溃会重复发布] → 明确采用 at-least-once；Redis revision max、全量 reload 和 cache invalidation 必须幂等，ack 使用 claim token 防止旧 owner 覆盖。
- [多个 batch 可能按不同速度产生乱序消息] → watcher 不按 revision gate 跳过消息副作用，tracker 和 Redis cache 只做 max；全量 reload 始终读取当前数据库状态。
- [持续 Redis 故障导致 outbox 增长] → 有界退避降低压力，保留 backlog/oldest-age metrics、状态与告警信号；不删除或设置不可恢复终态。
- [claim timeout 小于慢 publish 时 lease 被抢占] → 配置校验和默认值为正常 publish 留出预算；条件 ack 保护状态，重复副作用仍幂等。当前不实现 lease heartbeat，以保持协议最小。
- [坏 event 阻塞或反复重试] → 事件按独立 lease 处理，坏 event 进入 failed backoff，不阻塞后续 due event；metrics/log 暴露稳定错误类别，修复后可恢复。
- [滚动发布期间旧 watcher 无法解析新 payload] → 这是无 fallback 的 breaking 协议，部署必须先发布能消费新 payload 的 watcher，再启用 dispatcher publisher；回滚必须保持数据库事件不丢失。
- [新增 claim 字段需要受控 migration] → 先生成并应用 additive Atlas migration，再部署读取新字段的二进制；旧二进制忽略新增 nullable 列，可在切换期间继续运行。

## Migration Plan

1. 在 Ent schema 中增加 processing/failed 状态、claim token、lease 字段和 claim 扫描索引，运行 `make user-service-generate`、生成 additive Atlas migration 并执行 `make user-service-migrate-validate`。
2. 通过独立 migration Job 或受控发布平台先应用 migration；不修改已有 pending/delivered 事件语义，历史 pending event 在 dispatcher 启用后可直接处理。
3. 先滚动发布可解析新消息 envelope 且具备重复/乱序幂等行为的 watcher，但暂不启动 dispatcher；不提供旧 payload fallback。
4. 发布并启用 dispatcher 配置与 lifecycle，确认 backlog、oldest age、publish/retry metrics 和 readiness 状态正常，再移除在线 command 的 direct Redis publish 路径。
5. 回滚时先停用 dispatcher，再回滚应用；pending/failed/processing 事件保留在 PostgreSQL，processing lease 到期后可由恢复版本继续处理。新增列和状态不在紧急应用回滚中删除。

## Verification

- 运行 permission dispatcher/application 单元测试和 PostgreSQL 集成测试，覆盖 claim 竞争、lease 过期重领、stale token ack 拒绝、失败退避、重启恢复和 delivered 幂等。
- 运行 Redis publisher/watcher 测试，覆盖新 payload 校验、max revision、publish 各阶段失败、重复消息、乱序全局 reload 与定向 cache invalidation，并证明无旧 counter/payload fallback。
- 运行 lifecycle、配置、metrics 和 health 测试，证明 Start/Stop 幂等、停止期限、共享 client 所有权、只读探测和低基数标签契约。
- 运行 `make user-service-generate`、`make user-service-migrate-validate`、相关 Go package 测试与 `make user-service-architecture-lint`；若修改 dashboard/alert/runbook，再运行 `make compose-dashboard-check`。
- 确认没有 HTTP/OpenAPI 变化；完成预期暂存后运行 `make lint` 与 `make verify`。

## Open Questions

无。
