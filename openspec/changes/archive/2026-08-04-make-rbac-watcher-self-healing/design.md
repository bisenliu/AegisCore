## Context

当前 permission Redis watcher 在 `Start` 时创建一个后台循环。该循环先调用一次 `PubSub.Receive`，随后同时负责消费 `PubSub.Channel()` 和周期性 `CheckVersion`。初始 `Receive` 错误只会写入 `lastErr`，成功订阅或数据库校准不会清除它；channel 关闭则使整个循环退出，数据库 revision 补偿也随之停止。permission application 仅暴露 `Running()`/`LastError()`，observability 以任意历史错误直接判定 readiness/startup 失败，并通过通用 component collector 输出粘滞错误指标。

`go-redis/v9.21.0` 会在普通网络读取错误后尝试自动重连，但该隐式行为无法表达服务所需的当前订阅状态、退避进度和最后权威校准时间，也不能修复外层 channel 终止后的 watcher 生命周期。PostgreSQL latest policy revision 仍是权威恢复事实，Redis Pub/Sub 只允许作为快速唤醒 hint。

受影响路径包括 `user-service/internal/features/permission/`、`user-service/internal/config/`、`user-service/internal/providers/observability/`、Nacos 配置基线、`deployments/observability/`、Compose Grafana 生成物、测试说明和 OpenSpec。`common/` 的通用 component collector 不修改，watcher 改用 user-service 自有 collector；`internal/shared/` 与 `internal/integration/` 不承载本能力。

## Goals / Non-Goals

**Goals:**

- 让初始订阅失败、运行期 Receive 错误和底层订阅终止均在进程内自动恢复，不依赖人工或 Pod 重启。
- 保证订阅重建期间数据库 revision 补偿继续运行，并以最后一次成功权威校准时间表达同步新鲜度。
- 用结构化、可恢复的当前状态替代粘滞 `lastErr`，使 health、metrics、alerts 和 dashboard 区分运行、重连、已恢复和超出 staleness 预算。
- 使用可取消的有界指数退避、明确 goroutine 所有权和同步停止，避免忙重试、重复订阅和 goroutine 泄漏。
- 保持 PostgreSQL 权威、Casbin projection fail-closed、低基数指标和不暴露底层错误的安全边界。

**Non-Goals:**

- 不改变 RBAC mutation、policy revision、outbox、消息 envelope、Casbin policy 内容或用户角色缓存语义。
- 不新增数据库 schema、Atlas migration、HTTP API、OpenAPI 生成物或外部依赖。
- 不修改 Kubernetes/Helm probe 路径、liveness 语义或共享 Redis client 生命周期。
- 不为旧 `Running()`/`LastError()` 接口、旧 watcher component 指标或旧配置名称保留 adapter、双写、别名或回退分支。

## Decisions

### Decision: watcher 显式拥有订阅 supervisor 和权威校准循环

`Watcher.Start` 创建一个根 context 和一个根后台任务。根任务拥有订阅 supervisor、权威校准 ticker、内部 payload 队列和统一停止等待：

- 启动后立即执行一次权威校准，之后按 `check_interval` 周期执行。
- 订阅 supervisor 每次创建独立 PubSub，使用 `subscribe_timeout` 等待订阅确认，并通过低层 `Receive` 持续接收 Subscription、Message 和错误，不再使用 `PubSub.Channel()`。
- 初始确认或运行期 Receive 失败时，先关闭当前 PubSub、记录 subscription 当前错误，再按带抖动的有界指数退避创建新订阅；成功确认后清除 subscription 当前错误并重置退避。
- subscription supervisor 只把合法消息交给根循环；根循环串行执行 `HandlePayload` 与周期校准，保持现有 revision reload 和缓存失效不并发。
- 校准循环不依赖订阅连接状态。订阅正在退避时，PostgreSQL latest revision 查询和必要 reload 仍按期执行。

仅依赖 go-redis 隐式重连无法提供完整状态转换；让 channel 关闭后退出并依赖平台重启也不可接受，因为运行期 readiness 失败只会摘流，不会触发 liveness 重启。

### Decision: 权威校准成功具有严格定义

一次校准只有在以下条件全部成立时才更新 `LastReconcileSuccessAt`：成功读取 PostgreSQL latest policy revision；如本地 projection 未就绪或落后，reload/refresh 成功；最终 `ProjectionStatus` ready 且 applied revision 不低于本次数据库目标。数据库查询成功但 reload 失败不得刷新成功时间。

Pub/Sub 消息处理读取数据库失败时记录 reconcile 当前错误；下一次 Pub/Sub hint 或周期校准成功后清除。subscription 和 reconcile 使用独立当前错误类别，任一路恢复只清除自身错误，最近失败时间作为历史诊断保留。

### Decision: 以结构化快照替换旧状态接口

permission application 拥有 `PolicyWatcherStatusSnapshot` 和只读 `Status() PolicyWatcherStatusSnapshot` port。快照至少包含：

- `Running`
- 固定枚举的 `SubscriptionState`：`starting`、`connected`、`reconnecting`、`stopped`
- `LastSubscriptionSuccessAt`
- `LastReconcileSuccessAt`
- `LastFailureAt`
- 低基数 `SubscriptionErrorCategory` 和 `ReconcileErrorCategory`

底层错误只进入英文日志及稳定 `snake_case` 字段，不进入 status、metrics label 或公共健康响应。旧 `Running()`、`LastError()` 和 watcher 对 `common/runtime/observability/metrics.ComponentStatusCollector` 的使用直接删除。

### Decision: readiness 使用权威校准 staleness

新增 `rbac.policy_watcher` 配置：

- `check_interval`：默认 `15s`
- `subscribe_timeout`：默认 `5s`
- `max_staleness`：默认 `45s`
- `retry_backoff.initial`：默认 `250ms`
- `retry_backoff.max`：默认 `30s`

所有 duration 必须为正，最大退避不得小于初始退避，`max_staleness` 必须大于 `check_interval`。配置只接受这些正式名称，不读取旧名称或环境变量别名。

watcher health 在未运行、从未成功完成权威校准，或最后成功校准年龄大于 `max_staleness` 时 unavailable。处于 `reconnecting` 但权威校准仍新鲜时，watcher 自身 health 保持 available；聚合 readiness 仍可由独立 Redis health checker 表达 Redis 整体不可用。startup 在首次权威校准成功前失败。

### Decision: watcher 使用 feature 自有观测指标

permission/user-service 观测层提供低基数 watcher collector，至少输出：运行状态、订阅连接状态、最后订阅成功 timestamp、最后权威校准成功 timestamp、当前 staleness，以及按固定 result/reason 分类的重连尝试计数。删除 watcher 的 `aegiscore_runtime_component_running{resource="rbac_policy_watcher"}` 和 `aegiscore_runtime_component_last_error{resource="rbac_policy_watcher"}` 输出及其查询，不双写旧指标。

Prometheus 告警和 Grafana dashboard 同步改用新指标：停止或 staleness 超预算为 critical，持续重连为 warning，历史单次失败计数不直接造成持续健康失败。Compose dashboard 继续由通用 dashboard 生成，禁止手工制造两份漂移资产。

### Decision: 停止过程统一取消并等待全部后台任务

`Stop` 取消根 context，使 Receive、订阅确认 timeout、退避 timer、payload 发送和校准 ticker 全部可退出；当前 PubSub 只由 subscription supervisor 关闭一次，共享 Redis client 不关闭。根任务等待全部子任务结束后才关闭 `done` 并设置 `stopped`。正常取消不记录当前错误，取消后不得创建新订阅。

验证使用可控 subscriber、通道/barrier、明确 deadline 和 eventually 条件，不使用固定 `time.Sleep` 证明状态变化；运行 permission/observability/config 相关测试、race 检查、dashboard drift 检查、架构 lint、仓库 lint 和 verify。

## Risks / Trade-offs

- [Risk] subscription supervisor 与根循环停止竞争可能重复关闭 PubSub或阻塞 `Stop` → Mitigation：明确单一 PubSub owner，所有等待点同时监听根 context，并用 wait group 在关闭 `done` 前汇合。
- [Risk] payload 队列积压可能阻塞 Receive 并延迟订阅错误发现 → Mitigation：使用有界队列和 context-aware send，保持消息处理串行，并依靠周期权威校准覆盖被延迟或丢失的 hint。
- [Risk] `max_staleness` 过小会在短暂 PostgreSQL 抖动时频繁摘流，过大则延长不同步暴露窗口 → Mitigation：校验其大于检查周期，默认使用三个检查周期，并通过 staleness dashboard 与告警调参。
- [Risk] 删除旧 Prometheus series 会让未同步更新的告警和 dashboard 出现 No data → Mitigation：同一 change 更新两套 dashboard 资产、告警、runbook 和测试，并用生成/check 命令阻止 drift。
- [Risk] 新配置键会被旧二进制的严格配置解析拒绝 → Mitigation：先发布具备默认值的新二进制，再发布显式新配置；回滚时先移除新配置键，再回滚二进制，不引入代码兼容分支。

## Migration Plan

1. 完成 watcher、状态 port、health、metrics、配置和观测资产的单一版本替换，并通过全部故障注入与仓库验证。
2. 先发布使用默认 watcher 配置的新二进制；确认所有副本暴露新状态和指标后，再发布 Nacos 中的显式 `rbac.policy_watcher` 配置。
3. 更新 Prometheus rules、Grafana dashboard 和 runbook，确认 stopped、reconnecting、recovered 与 stale 场景均可定位。
4. 回滚时先从 Nacos 移除 `rbac.policy_watcher` 新键，再回滚二进制及观测资产；数据库和消息数据无需迁移或回滚。

## Open Questions

无。
