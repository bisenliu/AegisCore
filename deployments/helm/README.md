# Helm 部署

本目录承载 AegisCore 服务的 Helm charts。

当前状态：

- `aegiscore-user-services/` 预留给未来用户服务 chart。
- 当前没有提交可直接运行的 chart templates 或 values。

未来新增 charts 时，应将服务特定 templates 放在对应 chart 目录下，并使用由 `deployments/docker/user-service.Dockerfile` 构建的镜像。没有单独变更设计前，不要新增云厂商特定资源。

用户服务 Prometheus/Grafana 观测资产位于 `deployments/observability/`。当前这些资产作为独立示例维护，不要求本目录提供完整可生产运行的 chart templates。
