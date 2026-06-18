# Helm 部署

本目录承载 AegisCore 服务的 Helm charts。

当前状态：

- `aegiscore-user-services/` 预留给未来用户服务 chart。
- 当前没有提交可直接运行的 chart templates 或 values。

未来新增 charts 时，应将服务特定 templates 放在对应 chart 目录下，并使用由 `deployments/docker/user-service.Dockerfile` 构建的镜像。没有单独变更设计前，不要新增云厂商特定资源。

未来用户服务 chart 应提供独立 migration Job 配置边界，例如启用开关、镜像 tag 复用、`DATABASE_URL` Secret 引用和 Job 重试策略。Deployment 默认不启用 startup migration；发布顺序应是 migration Job 成功、RBAC seed 完成、再滚动用户服务副本。是否使用 Helm hook 需要结合具体发布平台另行设计，不在当前占位 chart 中预设。

用户服务 Prometheus/Grafana 观测资产位于 `deployments/observability/`。当前这些资产作为独立示例维护，不要求本目录提供完整可生产运行的 chart templates。
