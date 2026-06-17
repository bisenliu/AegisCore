# 可观测性资产

本目录承载用户服务第一版可执行运维观测资产。这些资产只消费应用已经落地的 metrics、日志和 tracing 上下文，不定义新的应用指标、云厂商资源或完整生产 Helm chart。

## 内容

| 路径 | 用途 |
|---|---|
| `grafana/user-service-overview.json` | 用户服务 Grafana 看板，覆盖 HTTP RED、auth/RBAC 信号、PostgreSQL、Redis、workerpool、scheduler、RBAC watcher、policy reload 和 Go runtime 面板。 |
| `prometheus/user-service-alerts.yaml` | 第一版生产告警基线的 Prometheus rule groups；同一个 `groups` block 可以复制到 Prometheus Operator 的 `PrometheusRule.spec.groups`。 |
| `../../docs/observability/user-service-runbook.md` | 告警 annotations 指向的稳定排障手册入口。 |

## 前提

- 用户服务通过 `observability.metrics.enabled: true` 启用 metrics。
- Prometheus 会抓取配置的 metrics path，默认 `/metrics`。
- Metrics endpoint 不经过 RBAC。请通过部署网络边界、Ingress 策略、service mesh 或等价控制保护暴露范围。
- Runtime collector 只使用低基数 label，例如 `service`、`environment`、`resource`、`pool`、`job`、`event`、`status` 和 `reason`。
- 本目录不提供 OpenTelemetry Collector、trace backend、云厂商 monitor、ServiceMonitor、PodMonitor 或完整 Helm chart。

## Grafana 导入

1. 打开 Grafana，导入 `grafana/user-service-overview.json`。
2. 按提示选择 Prometheus datasource。
3. 选择 `service` 和 `environment` 变量；默认 service 是 `aegiscore-user-services`。
4. 确认 HTTP RED、依赖、workerpool、scheduler、RBAC 和 runtime 面板能够加载。

如果某个面板为空，先确认 Prometheus 中是否存在对应指标。`observability.metrics.include_runtime: false` 时，Go runtime/process 面板可以为空。

## Prometheus 规则

本机有 `promtool` 时可校验规则文件：

```bash
promtool check rules deployments/observability/prometheus/user-service-alerts.yaml
```

在 Prometheus Operator 环境中，可将 groups 包装为 `PrometheusRule` 对象：

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: aegiscore-user-service-alerts
spec:
  groups:
    # 将 prometheus/user-service-alerts.yaml 中的 groups 复制到这里
```

`readyz` 告警依赖外部探测指标，例如 blackbox exporter 的 `probe_success` 或等价 Kubernetes probe metrics。仅靠应用 `/metrics` 不能证明 `/readyz` 正在失败。

## 本地验证

1. 启用 metrics：

```bash
AEGISCORE_OBSERVABILITY_METRICS_ENABLED=true make run-user-service
```

2. 确认 metrics endpoint：

```bash
curl -fsS http://localhost:8080/metrics | head
```

3. 在 Prometheus 中执行看板或告警文件里的关键表达式。
4. 导入 Grafana 看板，并确认变量值可用。
5. 加载告警规则文件；也可以在非生产环境临时降低阈值，验证告警触发路径。

## Tracing 说明

本地 tracing 默认使用 `observability.tracing.exporter: none`。在这种模式下，服务仍会创建 trace/span context，日志也可以包含 `trace_id` 和 `span_id`，但 span 不会被导出，也不会出现在 trace UI 中。Trace 可视化需要 OpenTelemetry Collector 和 trace backend，并将 `observability.tracing.exporter=otlp`、`observability.tracing.otlp_endpoint` 指向 Collector。
