## 1. Compose 编排

- [x] 1.1 为 Prometheus、Grafana 增加 `observability` profile，并让 `rbac-seed` 复用 user-service 镜像；保留现有 `latest` 标签和 Nacos 双初始化构建。
- [x] 1.2 检查默认、`observability` 和 `seed` profile 渲染的服务及镜像，并验证 Compose 配置合法。

## 2. 文档与验证

- [x] 2.1 更新 `deployments/compose/README.md`、Compose 命令注释和指标脚本说明，说明按需观测及 seed 镜像前提。
- [x] 2.2 运行 `openspec validate`、`make compose-dashboard-check` 和 `make user-service-architecture-lint`，确认规格与部署资产一致。
- [x] 2.3 暂存本次预期变更，运行 `make lint` 与 `make verify` 并检查最终 diff。
