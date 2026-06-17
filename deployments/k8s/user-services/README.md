# user-services Kubernetes 清单

本目录预留给未来用户服务运行时的 Kubernetes 清单。

当前没有提交可直接运行的清单。只有在变更中明确部署形态、必需配置、密钥处理方式和验证步骤后，才新增清单。

未来新增用户服务 Pod 探针时，应使用：

- `GET /livez` 作为 liveness probe。
- `GET /readyz` 作为 readiness probe。
- `GET /startupz` 作为 startup probe。

Prometheus/Grafana dashboard、alert rule 示例和 runbook 链接位于 `deployments/observability/`；本目录未来可在单独变更中接入 ServiceMonitor、PodMonitor 或 blackbox readyz probe。
