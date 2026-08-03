# User Service Observability Runbook

## Metrics Endpoint

用户服务在 `observability.metrics.enabled: true` 时暴露配置化 Prometheus scrape endpoint，默认 `/metrics`。该 endpoint 不进入 `/api/v1`，不经过 RBAC 授权，必须由部署、网关或网络策略保护。

## Key Signals

- HTTP RED：请求数、延迟 histogram、in-flight gauge，route label 使用 Gin route template。
- PostgreSQL：`primary_db` pool stats 和可用性。
- Redis：`cache_redis` ping/up 状态。
- Localcache：`auth_token_version` 与 `rbac_user_roles` 的 hit/miss、loader success/error、自动 eviction 和最大 item capacity。
- Runtime components：workerpool、scheduler、runtime component status。
- RBAC：policy watcher 状态、Casbin policy reload 成功/失败、policy sync 版本 mismatch、policy reload lag。
- Business metrics：auth login/refresh/logout、token version mismatch、route diff missing/stale 等固定低基数指标。

## Troubleshooting

- HTTP 5xx 升高：先按 route template 聚合定位，再结合 `trace_id` / `span_id` 查日志；不得依赖 raw path 高基数标签。
- 延迟升高：检查 HTTP p95、PostgreSQL wait/pool stats、Redis ping 和 downstream runtime 指标。
- `/readyz` 或 `/startupz` 失败：检查 PostgreSQL、Redis、Casbin policy state 和 RBAC watcher state。
- Goroutine 数量过高：检查 Go runtime dashboard、`/debug/pprof/goroutine`、近期发布变更和后台任务状态，区分真实泄漏与短时并发堆积。
- RBAC watcher stopped：检查 Redis Pub/Sub、数据库 policy revision compensation、实例日志和 watcher metrics；授权热路径不会每请求读取数据库或 Redis revision。
- Casbin reload failed：检查权限目录、角色状态、绑定关系和 policy loader 错误；不要通过 route diff 写入权限。
- RBAC policy reload lag：在线 RBAC 写成功后，数据库和 watcher 正常运行时其他副本应在 30 秒内完成 policy 最终生效；持续 lag 表示至少一个副本本地 Casbin applied projection revision 落后数据库 latest policy revision。
- Workerpool failed/panicked：定位 owning feature 的后台清理任务，workerpool 只是 runtime executor，不承担业务补偿语义。

## Runtime Alerts

### goroutine-count

`go_goroutines` 表示当前进程 goroutine 数量。值超过 `10000` 并持续 5 分钟时，可能存在 goroutine 泄漏、外部依赖阻塞导致的请求堆积、后台任务卡住或突发流量超过实例处理能力。

先确认 `observability.metrics.include_runtime: true` 且 Prometheus 正常 scrape 当前实例，再查看 Go runtime dashboard 的 goroutine、heap、GC 和 process memory 趋势。随后通过受控 pprof 入口采集 goroutine profile，按栈聚合定位增长最快的调用路径，并结合近期 deployment、HTTP in-flight、PostgreSQL pool wait、Redis up/ping 和 workerpool/scheduler 告警判断是否为依赖阻塞或代码泄漏。

不要通过重启实例作为唯一处置；如果重启后 goroutine 数量随流量持续线性增长，应保留 profile、日志和变更窗口，优先回滚最近涉及 goroutine 生命周期、请求上下文取消、Redis Pub/Sub、scheduler 或 workerpool 的发布。

## RBAC Alerts

### rbac-policy-reload-lag

`aegiscore_user_service_rbac_policy_reload_lag` 表示 watcher 最近一次成功读取的数据库 latest RBAC policy revision 与本地 Casbin engine 实际 applied projection revision 的非负差值。值大于 `0` 持续 30 秒时，权限变更可能尚未在该实例最终生效；新增权限可能继续被拒绝，移除权限或禁用角色可能在落后实例上短暂继续允许。lag 为 `0` 只在成功数据库校准证明本地 applied revision 不小于该次 database latest 时成立。

优先检查同一实例的 `rbac_policy_watcher` runtime component 是否 running、`aegiscore_runtime_component_last_error{resource="rbac_policy_watcher"}` 是否为 1、PostgreSQL `primary_db` 可用性、`aegiscore_user_service_rbac_policy_sync_operations_total{operation="watcher_revision_check",result="failure",reason="revision_store_unavailable"}` 是否增加，以及 `aegiscore_casbin_policy_reloads_total{status="failure"}` 是否增加。revision source 查询失败时 lag gauge 保留上一次成功校准值，不得把保留值 `0` 当作当前数据库已收敛证明。依赖恢复后，watcher 的下一条 Pub/Sub hint 或周期性 database revision compensation 应重新读取 database latest、reload policy并把 lag 收敛到 0。

Redis Pub/Sub 只提供唤醒 hint，消息允许丢失、重复、乱序；Redis counter 不存在、落后或重建不会参与补偿判断。遇到旧消息时确认日志中的 `hint_revision` 与 `database_latest_policy_revision`，实际 reload target 必须来自数据库。授权热路径不会每请求读取数据库或 Redis revision，不要通过要求业务接口强一致回源来处理该告警；如果 lag 持续且数据库、watcher、reload 均正常，检查最近部署是否改变了 policy loader、revision source、Pub/Sub channel 或权限/角色绑定写后通知路径。

## Localcache Alerts

### localcache-load-errors

`aegiscore_localcache_loads_total{result="error"}` 增加表示本地缓存 miss 后的 loader 回源失败。先按 `cache` label 区分来源：`auth_token_version` 通常要检查 Redis token version 投影、PostgreSQL 用户凭据读取和 auth validator 日志；`rbac_user_roles` 通常要检查角色绑定查询、Casbin user role resolver 和 PostgreSQL 访问路径。

### password-change-revocation-failed

`aegiscore_user_service_auth_password_change_revocation_projection_failures_total` 或 `aegiscore_user_service_auth_password_change_revocation_compensation_failures_total` 增加表示强制改密已更新凭据但安全撤销未完整完成。优先检查 Redis `cache_redis` 可用性、auth token version 投影刷新、本地 token version cache 失效、refresh session 删除链路和 auth feature 错误日志；不要在排查记录中复制 token、jti、session ID 或 Redis key 明文。

如果同时出现 Redis 或 PostgreSQL 告警，优先处理依赖不可用；如果依赖正常，检查最近部署是否改变了 loader、TTL、容量或权限/认证数据路径。该指标不包含 raw key、用户 ID 或原始错误，定位具体请求需要结合日志中的稳定错误信息和 trace/span 上下文。

### localcache-eviction-pressure

`aegiscore_localcache_evictions_total` 相对 `aegiscore_localcache_capacity` 快速增长表示缓存淘汰压力升高。先确认是否是冷启动或批量流量导致的短暂加载，再检查对应 cache 的 hit ratio、loader error 和依赖延迟。

如果淘汰压力持续存在且 hit ratio 下降，优先评估容量、TTL 和 key 基数；如果淘汰压力升高但 hit ratio 稳定，可能只是访问集大于容量预算，需要结合资源成本决定是否扩容。

## Safety Rules

Metrics label 必须低基数，不得包含 user ID、role ID、permission ID、session ID、token ID、trace ID、span ID、raw path、IP、邮箱、用户名、SQL、Redis key 或原始错误。日志不得记录 password、token、Authorization header、Cookie、原始请求体、DSN、SQL 或 Redis key。
