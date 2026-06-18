# User Service Observability Runbook

## Metrics Endpoint

用户服务在 `observability.metrics.enabled: true` 时暴露配置化 Prometheus scrape endpoint，默认 `/metrics`。该 endpoint 不进入 `/api/v1`，不经过 RBAC 授权，必须由部署、网关或网络策略保护。

## Key Signals

- HTTP RED：请求数、延迟 histogram、in-flight gauge，route label 使用 Gin route template。
- PostgreSQL：`user_db` pool stats 和可用性。
- Redis：`cache_redis` ping/up 状态。
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

## Safety Rules

Metrics label 必须低基数，不得包含 user ID、role ID、permission ID、session ID、token ID、trace ID、span ID、raw path、IP、邮箱、用户名、SQL、Redis key 或原始错误。日志不得记录 password、token、Authorization header、Cookie、原始请求体、DSN、SQL 或 Redis key。
