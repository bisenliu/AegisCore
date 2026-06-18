# aegiscore-user-services Helm Chart

本目录预留给未来运行时名称为 `aegiscore-user-services` 的用户服务 Helm chart。

当前没有提交 Chart 元数据、配置值或模板。只有在变更中明确支持的部署配置和验证步骤后，才新增这些内容。

未来新增 chart templates 时，应将数据库 migration 建模为独立 Job，而不是用户服务 Deployment 的默认启动动作。建议预留 `migrationJob.enabled`、image、command、Secret 引用和重试策略等配置边界；Job command 为 `/app/user-service/scripts/migrate-apply.sh`，且默认使用当前发布镜像。Deployment 默认不设置 `RUN_MIGRATIONS=true`。

未来新增 chart probe values 或 templates 时，应将 liveness、readiness 和 startup probe 分别指向 `/livez`、`/readyz` 和 `/startupz`。

Prometheus/Grafana dashboard、alert rule 示例和 runbook 链接位于 `deployments/observability/`；本 chart 目录当前不封装这些资产。
