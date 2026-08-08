## Why

permission watcher 当前自行实现 Redis Pub/Sub 的订阅确认、接收、退避重连、缓冲背压和关闭并发控制，使业务适配器同时承担通用连接生命周期与 RBAC 权威校准语义。将这组业务中立且可复用的行为收敛到 `common/runtime/redispubsub`，可以统一并发与资源所有权契约，同时让 watcher 测试只关注 RBAC 行为。

## What Changes

- 在 `common/runtime/redispubsub` 新增 Redis 单 channel subscription primitive，提供严格校验的显式配置、订阅确认、阻塞接收、有界缓冲、带抖动的有界指数退避重连、结构化状态以及单向启停生命周期。
- 明确 primitive 仅提供 Redis classic Pub/Sub 的 at-most-once 通知接收，不承担 publish 业务封装、消息 envelope、revision/outbox/幂等、数据库校准、Casbin reload、缓存失效、模式订阅、sharded Pub/Sub、Redis Streams 或可靠投递。
- permission provider 使用服务配置显式构造 `redispubsub.Subscriber`，其中消息缓冲固定传入 `64`；permission watcher 通过 feature-local 最小 `messageSource` 接口消费消息，并继续拥有 PostgreSQL revision 权威校准、消息解码、reload、缓存失效与 RBAC 状态组合。
- 从 permission Redis store 和 watcher 删除订阅 client 暴露、订阅 supervisor、attempt 所有权、退避算法与订阅状态直写逻辑；`Store` 只暴露业务 channel 名称供 composition 使用。
- **BREAKING**：`Watcher.Start()` 改为返回 `error`，Fx lifecycle 与全部调用点一次性迁移；不提供 deprecated alias、wrapper、旧构造器或兼容分支。
- 将通用订阅故障与生命周期测试迁入 `common/runtime/redispubsub`，permission watcher 测试仅保留 RBAC 业务处理、权威校准、周期补偿与组合状态。
- 同步更新架构文档、能力地图以及 `shared-platform-primitives` 与 `rbac-access-control` 规格。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`：新增通用 Redis classic Pub/Sub 单 channel subscriber 的配置、重连、背压、状态与资源生命周期契约。
- `rbac-access-control`：RBAC watcher 改为复用通用 subscriber，并调整启动、停止、配置归属和组合状态契约，同时保持 PostgreSQL 权威与业务处理语义不变。

## Impact

- 代码：新增 `common/runtime/redispubsub/`；修改 `user-service/internal/features/permission/infrastructure/redis/`、permission provider/Fx lifecycle 及相关测试与配置接线。
- 共享契约：新增导出的 `Subscriber`、`Options`、`Message`、`Status`、`State` 和 `ErrorCategory` API；调用方必须显式提供全部 option，不由 primitive 注入默认值。
- 运行时与可观测性：订阅状态改由通用 primitive 提供，RBAC watcher 映射并组合现有 application 状态；日志继续使用结构化低基数字段，Redis Pub/Sub 的 at-most-once 语义不变。
- 安全与一致性：PostgreSQL policy revision 仍是权威来源，Casbin fail-closed、周期补偿、reload 和缓存失效语义不变；不扩大 `common` 到 RBAC 业务语义。
- 外部接口与交付：不修改 HTTP API、数据库 schema、migration、OpenAPI 生成物、部署清单或观测资产，也不关闭或改变共享 Redis client 的所有权。
- 文档与规格：更新 `docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 和两个既有 capability 的 OpenSpec delta。
