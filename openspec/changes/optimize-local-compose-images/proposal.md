## Why

本地 Compose 默认启动全部观测服务，并在多个一次性任务上重复声明同名镜像的构建；反复构建容易留下无标签镜像，也增加启动和维护成本。

## What Changes

- Prometheus 和 Grafana 改为通过 `observability` profile 按需启动，基础服务和 Jaeger 保持默认启动。
- 让 RBAC seed 复用 user-service 镜像，减少重复构建；保留 Nacos 双初始化服务的独立构建能力。
- 更新默认启动和按需观测说明；上游镜像继续使用 `latest`。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `delivery-operations`: 明确本地 Compose 的默认服务和可选观测 profile。

## Impact

影响 `deployments/compose/`、相关开发文档和观测脚本用法。默认 `up -d` 不再启动 Prometheus/Grafana；需要看板时须显式启用 profile。不改变上游镜像标签、Go 代码、HTTP API、数据库 schema、SQL migration 或生产 Helm/Kubernetes 部署。
