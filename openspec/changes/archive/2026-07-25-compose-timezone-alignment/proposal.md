## Why

当前本地 Compose 中只有 Nacos 与 user-service 明确表现为 `Asia/Shanghai`，PostgreSQL、Redis、Jaeger、Prometheus 和 Grafana 的进程或日志仍使用 UTC。`jaegertracing/all-in-one:latest` 当前解析为已停止维护的 Jaeger 1.76 且镜像不包含 IANA zoneinfo，单独设置 `TZ=Asia/Shanghai` 不会生效，导致本地跨组件排障时需要人工换算时间。

需要统一本地 Compose 的进程与日志时区，并明确观测系统的展示和存储边界，避免把容器时区、浏览器时区以及基于 Unix epoch 的 trace/metric 时间戳混为一谈。

## What Changes

- 为 Compose 中的常驻服务与一次性任务显式设置 `TZ=Asia/Shanghai`。
- 为缺少 IANA zoneinfo 的 Jaeger 1.76 基础镜像增加仅包含所需时区数据的本地薄镜像，使 Go 进程实际加载 `Asia/Shanghai`。
- 为 PostgreSQL 显式设置 session 与日志时区，确保已有 data volume 重启后也使用 `Asia/Shanghai`，不依赖初始化时自动推导。
- 将 Compose Grafana 的默认 dashboard 展示时区固定为 `Asia/Shanghai`，并保持源 dashboard 与 provisioning 副本同步。
- 记录 Prometheus Web UI 固定使用 UTC、Jaeger UI 使用浏览器本地时区、OpenTelemetry 与 Prometheus 内部时间继续使用 Unix epoch/UTC 的边界。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `delivery-operations`：Compose 服务、一次性任务、PostgreSQL 和 Jaeger 本地镜像必须提供可验证的上海时区运行语义。
- `runtime-observability`：本地观测组件的日志、Grafana 默认展示时区以及 Jaeger/Prometheus 展示边界必须明确且可验证。

## Impact

- 部署：修改 `deployments/compose/docker-compose.yml`，新增 Jaeger 本地 Dockerfile，并重建相关 Compose 容器。
- 观测资产：修改 Grafana dashboard 源文件并重新生成 Compose provisioning 副本。
- 文档：更新 Compose 与观测文档中的时区说明和例外边界。
- 安全与数据：不新增 secret，不删除或重建 volume，不改变 PostgreSQL schema、Atlas migration、HTTP API、OpenAPI 或 RBAC 行为。
- 兼容性：trace 与 metric 时间戳继续以标准绝对时间存储；Prometheus Web UI 的 UTC 展示和 Jaeger UI 的浏览器本地时间展示不做非官方覆盖。
