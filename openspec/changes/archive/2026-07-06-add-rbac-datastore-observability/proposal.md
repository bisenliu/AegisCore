## Why

当前 RBAC 授权热路径、Redis 命令和 Ent 查询缺少可直接定位 allow/deny/error、依赖调用耗时和错误的观测信号，线上排查只能依赖 HTTP 总体指标、连接池指标或日志，无法区分授权拒绝、授权异常、Redis 命令耗时和 Ent 查询异常。

本次变更补齐低基数 metrics 与 OpenTelemetry tracing，确保 SRE 和开发者可以在不暴露用户 ID、SQL、Redis key 或原始错误的前提下定位 RBAC 与 datastore 热路径问题。

## What Changes

- 为 RBAC Enforce 增加 allow、deny、error counter 和 latency histogram，标签限定为 `result`、`method`、`route_template`。
- 为 go-redis client 统一安装 OpenTelemetry hook，使 Redis 命令在已有 trace context 下产生 client span。
- 为 Ent 查询增加 query span、query latency histogram 和 query error counter，使用稳定低基数 query/entity/result 标签，不记录 raw SQL、参数或用户标识。
- **BREAKING** 不保留无观测的 Redis/Ent/RBAC 热路径，不提供旧指标名、旧标签或兼容 PromQL；实现和观测资产直接消费新指标与 span 语义。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 扩展 runtime metrics 与 tracing 契约，覆盖 Redis 命令 tracing、Ent 查询 tracing、Ent 查询 metrics 和低基数标签约束。
- `rbac-access-control`: 扩展 RBAC 授权保护契约，要求每次 Enforce 判定导出 allow/deny/error 结果与耗时指标。

## Impact

- 影响代码：`user-service/internal/features/permission/application/authorization`、`user-service/internal/features/permission` metrics provider、`common/runtime/datastore` Redis client 构造、`user-service/internal/providers/ent.go`、`common/runtime/observability/metrics` 或服务级 metrics collector。
- 影响依赖：需要引入 go-redis OpenTelemetry instrumentation；Ent query tracing/metrics 可通过 Ent interceptor、driver wrapper 或服务级 provider 实现。
- 影响 metrics：新增 RBAC Enforce 和 Ent query 指标；现有 HTTP、SQL pool、Redis ping、localcache、workerpool、scheduler 指标语义不变。
- 影响 tracing：Redis 命令和 Ent 查询在 tracing 启用时应出现在同一请求 trace 中；tracing 禁用时不得产生业务副作用。
- 影响部署观测：Prometheus/Grafana/alerts 或 metrics load 验证需要消费新增指标，不保留旧指标兼容查询。
- 不影响外部 HTTP API、OpenAPI、数据库 schema、Atlas migration、RBAC 授权业务结果或 Redis key schema。
