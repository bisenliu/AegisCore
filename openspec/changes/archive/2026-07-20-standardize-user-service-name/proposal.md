## Why

当前 user-service 的外部服务标识存在 `aegiscore-user-services`、`aegiscore-user-service` 和 `user-services` 混用。该问题已经扩散到 CLI、镜像、Kubernetes/Helm 资源、metrics `service` label、Prometheus/Grafana 查询、JWT issuer 和 Redis key prefix，继续保留会增加发布、告警和跨环境排障时的漂移风险。

本次变更将 user-service 的外部服务标识统一为单数 `aegiscore-user-service`，与仓库目录、Go module、Make target 和服务语义保持一致，并以一次 breaking change 清除旧复数契约。

## What Changes

- **BREAKING**: CLI 根命令从 `aegiscore-user-services` 改为 `aegiscore-user-service`，不保留旧命令别名。
- **BREAKING**: Docker/CI/Compose/Kubernetes/Helm 默认镜像名、容器内二进制名、工作负载名称、ServiceAccount、ConfigMap、Secret、Job、PDB、HPA、NetworkPolicy、Helm chart 名称和 release 示例统一使用 `aegiscore-user-service` 或 `user-service`，不保留旧资源名兼容。
- **BREAKING**: 默认 `app.name`、日志 `service` 字段、metrics `service` label、tracing `service.name`、Prometheus alert 查询、Grafana dashboard 默认变量和 Compose scrape label 统一为 `aegiscore-user-service`。
- **BREAKING**: JWT issuer 和 Redis key prefix 统一为 `aegiscore-user-service`，旧 issuer、旧 Redis key prefix 不做兼容读取或双写。
- 更新 OpenSpec 主规格 delta、文档、测试断言和生成物，确保交付、观测、认证和 RBAC 运维契约只保留单数服务名。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `delivery-operations`: 修改 user-service CLI、镜像、Compose、Kubernetes、Helm 和发布资产的稳定命名契约。
- `runtime-observability`: 修改日志、metrics、tracing、健康响应、Prometheus/Grafana 观测资产使用的稳定服务标识。
- `auth-session-management`: 修改 JWT issuer 和 auth Redis key prefix 使用的服务标识契约。
- `rbac-access-control`: 修改 RBAC 运维 CLI 示例、policy sync Redis key prefix 和相关观测/发布契约。

## Impact

- Go 代码：`user-service/cmd`、`internal/config`、`internal/bootstrap`、`internal/providers`、auth/permission Redis key tests、metrics tests 和相关 fixture 需要同步更新。
- 部署资产：`deployments/docker`、`.github/workflows`、`deployments/compose`、`deployments/k8s`、`deployments/helm`、`deployments/observability`、Grafana dashboard 副本和 Prometheus rules 需要统一命名。
- 文档和规格：`docs/`、`openspec/specs/`、OpenAPI 示例和 runbook/README 中的服务标识需要同步。
- 安全与运行时：JWT issuer 改名会使旧 issuer token 不再被当前配置接受；Redis key prefix 改名会使旧 refresh session、password-change session、token version projection 和 RBAC policy version/channel 不再被读取。
- 发布影响：Kubernetes/Helm 资源名改动会创建新资源并删除旧资源；Prometheus 历史序列按新的 `service` label 重新开始；镜像仓库和 CLI 调用方必须一次性切换。
