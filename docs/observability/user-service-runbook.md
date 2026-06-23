# User Service Observability Runbook

## Metrics Endpoint

用户服务在 `observability.metrics.enabled: true` 时暴露配置化 Prometheus scrape endpoint，默认 `/metrics`。该 endpoint 不进入 `/api/v1`，不经过 RBAC 授权，必须由部署、网关或网络策略保护。

## Key Signals

- HTTP RED：请求数、延迟 histogram、in-flight gauge，route label 使用 Gin route template。
- PostgreSQL：`user_db` pool stats 和可用性。
- Redis：`cache_redis` ping/up 状态。
- Localcache：`auth_token_version` 与 `rbac_user_roles` 的 hit/miss、loader success/error、singleflight、write drop/reject、eviction 和 capacity。
- Runtime components：workerpool、scheduler、runtime component status。
- RBAC：policy watcher 状态、Casbin policy reload 成功/失败、policy sync 版本 mismatch。
- Business metrics：auth login/refresh/logout、token version mismatch、route diff missing/stale 等固定低基数指标。

## Troubleshooting

- HTTP 5xx 升高：先按 route template 聚合定位，再结合 `trace_id` / `span_id` 查日志；不得依赖 raw path 高基数标签。
- 延迟升高：检查 HTTP p95、PostgreSQL wait/pool stats、Redis ping 和 downstream runtime 指标。
- `/readyz` 或 `/startupz` 失败：检查 PostgreSQL、Redis、Casbin policy state 和 RBAC watcher state。
- RBAC watcher stopped：检查 Redis Pub/Sub、policy version compensation、实例日志和 watcher metrics；授权热路径不会每请求读 Redis 强一致校验。
- Casbin reload failed：检查权限目录、角色状态、绑定关系和 policy loader 错误；不要通过 route diff 写入权限。
- Workerpool failed/panicked：定位 owning feature 的后台清理任务，workerpool 只是 runtime executor，不承担业务补偿语义。

## Localcache Alerts

### localcache-load-errors

`aegiscore_localcache_loads_total{result="error"}` 增加表示本地缓存 miss 后的 loader 回源失败。先按 `cache` label 区分来源：`auth_token_version` 通常要检查 Redis token version 投影、PostgreSQL 用户凭据读取和 auth validator 日志；`rbac_user_roles` 通常要检查角色绑定查询、Casbin user role resolver 和 PostgreSQL 访问路径。

如果同时出现 Redis 或 PostgreSQL 告警，优先处理依赖不可用；如果依赖正常，检查最近部署是否改变了 loader、key 编码、TTL 或权限/认证数据路径。该指标不包含 raw key、用户 ID 或原始错误，定位具体请求需要结合日志中的稳定错误信息和 trace/span 上下文。

### localcache-write-drops-or-rejects

`aegiscore_localcache_writes_total{event="set_dropped"}` 表示写入队列丢弃，`event="rejected"` 表示 Ristretto admission 拒绝。先查看 dashboard 中对应 cache 的 `capacity`、hit ratio、load rate 和 key 访问模式；持续增长通常意味着容量过小、TTL 不合适、key 基数变高或写入突增。

短时间内少量 `rejected` 可能是 admission policy 的正常行为；持续 `set_dropped` 或 rejected 与 loader rate 同时升高时，应评估 `local_cache.<name>.capacity`、`num_counters`、`buffer_items` 和业务流量变化。

### localcache-eviction-pressure

`aegiscore_localcache_evictions_total` 相对 `aegiscore_localcache_capacity` 快速增长表示缓存淘汰压力升高。先确认是否是冷启动或批量流量导致的短暂预热，再检查对应 cache 的 hit ratio、loader error、write drop/reject 和依赖延迟。

如果淘汰压力持续存在且 hit ratio 下降，优先评估容量、TTL 和 key 基数；如果淘汰压力升高但 hit ratio 稳定，可能只是访问集大于容量预算，需要结合资源成本决定是否扩容。

## Safety Rules

Metrics label 必须低基数，不得包含 user ID、role ID、permission ID、session ID、token ID、trace ID、span ID、raw path、IP、邮箱、用户名、SQL、Redis key 或原始错误。日志不得记录 password、token、Authorization header、Cookie、原始请求体、DSN、SQL 或 Redis key。
