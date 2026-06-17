# aegiscore-user-services Helm Chart

本目录预留给未来运行时名称为 `aegiscore-user-services` 的用户服务 Helm chart。

当前没有提交 Chart 元数据、配置值或模板。只有在变更中明确支持的部署配置和验证步骤后，才新增这些内容。

未来新增 chart probe values 或 templates 时，应将 liveness、readiness 和 startup probe 分别指向 `/livez`、`/readyz` 和 `/startupz`。

Prometheus/Grafana dashboard、alert rule 示例和 runbook 链接位于 `deployments/observability/`；本 chart 目录当前不封装这些资产。
