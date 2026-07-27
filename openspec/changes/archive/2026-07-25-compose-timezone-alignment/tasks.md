## 1. Compose 与镜像

- [x] 1.1 新增只补 `Asia/Shanghai` zoneinfo 的 Jaeger 薄镜像，并保持基础镜像 entrypoint 与功能不变。
- [x] 1.2 为 Compose 常驻服务和一次性任务统一注入 `TZ=Asia/Shanghai`，为 PostgreSQL 固定 `timezone` 与 `log_timezone`。
- [x] 1.3 更新 Compose 常用命令和镜像清理说明，包含本地 Jaeger image。

## 2. 观测资产与文档

- [x] 2.1 将 Grafana dashboard 默认时区改为 `Asia/Shanghai`，运行生成脚本同步 Compose provisioning 副本并执行 `make compose-dashboard-check`。
- [x] 2.2 更新 Compose 与观测文档，说明 Jaeger UI、Prometheus UI、Grafana 和 telemetry 时间语义边界。

## 3. 验证

- [x] 3.1 运行 `docker compose config --quiet`、`openspec validate compose-timezone-alignment` 和 `make user-service-architecture-lint`。
- [x] 3.2 使用 `docker compose up -d --build` 保留 volume 重建服务，验证全部 `TZ`、Jaeger zoneinfo、PostgreSQL `show timezone`、各组件 UTC+8 日志及 user-service healthcheck。
- [x] 3.3 暂存本次预期 Compose、Docker、观测、文档和 OpenSpec 变更后运行 `make lint` 与 `make verify`，确认无生成物 drift。
