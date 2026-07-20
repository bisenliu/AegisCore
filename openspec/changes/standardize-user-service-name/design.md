## Context

user-service 当前将同一个外部服务标识分散表达为 `aegiscore-user-services`、`aegiscore-user-service`、`user-services` 和目录语义 `user-service`。其中 `aegiscore-user-services` 已成为 CLI、默认 `app.name`、Kubernetes/Helm 资源、镜像名、JWT issuer、Redis key prefix 和 Prometheus 查询的主流运行时契约；`aegiscore-user-service` 又出现在 Prometheus rule group、Grafana UID、Compose job name 和测试 fixture 中。

本次 change 的目标不是修复单点故障，而是以一次不保留兼容的 breaking change 消除未来扩散风险。变更涉及 `common` 观测 primitive 的消费方式、`user-service` 运行时配置和测试、`deployments` 交付资产，以及 `docs/openspec` 中对稳定契约的描述。

## Goals / Non-Goals

**Goals:**

- 将 user-service 对外服务标识统一为 `aegiscore-user-service`。
- 将容器内二进制和容器名称统一为 `user-service`。
- 同步 CLI、默认配置、Docker、CI、Compose、Kubernetes、Helm、Prometheus、Grafana、OpenAPI 示例、文档和 OpenSpec 主规格。
- 明确不保留旧 CLI 名称、旧镜像名、旧 Kubernetes/Helm 资源名、旧 metric label、旧 JWT issuer 或旧 Redis key prefix。
- 通过测试和 lint 验证新命名在运行时、观测和交付资产中一致。

**Non-Goals:**

- 不迁移历史 Prometheus 时间序列。
- 不迁移或读取旧 Redis key prefix 下的 refresh session、password-change session、token version projection 或 RBAC policy sync key。
- 不保留 `aegiscore-user-services` CLI alias、镜像 alias、Helm chart alias 或 Kubernetes shadow resource。
- 不改变 HTTP API path、数据库 schema、Ent schema、OpenAPI 路由语义或业务授权行为。
- 不把服务命名常量下沉到 `common`；`common` 继续只消费调用方传入的服务名。

## Decisions

### Decision: 唯一外部服务名使用 `aegiscore-user-service`

选择单数 `aegiscore-user-service` 作为唯一外部服务标识。该名称与仓库目录 `user-service/`、Go module `github.com/aegiscore/user-service`、Make target 前缀 `user-service-*` 和单一服务语义一致。

备选方案是继续使用复数 `aegiscore-user-services` 并清理单数引用。该方案可以减少当前默认配置改动，但会继续与目录、module 和服务语义不一致，并要求后续新增资产理解“一个服务使用复数名”的历史原因，因此不采用。

### Decision: 不保留兼容路径

CLI、Docker image、Kubernetes/Helm 资源、JWT issuer、Redis key prefix 和 metrics `service` label 均直接改名，不提供 alias、双写、双读或兼容 PromQL。

备选方案是分阶段双写或保留别名。该方案降低单次发布风险，但会让旧名继续存在于安全、观测和交付契约中，无法完成命名收敛，因此不采用。

### Decision: 服务名仍由 user-service 配置拥有

`common/runtime/observability`、`common/runtime/logger` 和 tracing provider 继续只接收 `cfg.App.Name` 或显式 `ServiceName`。本次只修改 user-service 默认配置、测试 fixture 和部署注入值，不在 `common` 增加 user-service 专属常量。

备选方案是在 `common` 提供共享服务名常量。该方案违反 `common` 不承载 user-service 业务或部署语义的边界，因此不采用。

### Decision: 交付资产和文档同步改名

`deployments/docker`、`.github/workflows`、`deployments/compose`、`deployments/k8s`、`deployments/helm`、`deployments/observability` 和 `docs/` 必须在同一实现中同步，避免代码、镜像、清单和告警查询出现中间态。

备选方案是先改代码再改部署，或先改观测再改镜像。该方案会在本地和 CI 中引入短期漂移，不符合本次一次性 breaking change 的目标，因此不采用。

## Risks / Trade-offs

- [Risk] 旧 JWT issuer token 不再通过当前配置校验 -> Mitigation：发布说明明确这是 breaking change，发布后用户重新登录获取新 token。
- [Risk] 旧 Redis key prefix 下的会话和 RBAC policy sync key 不再消费 -> Mitigation：发布时接受旧会话失效和副本通过新 key 重新收敛，必要时由运维在维护窗口清理旧 key。
- [Risk] Kubernetes/Helm 资源名变化导致新旧资源并存或切换窗口 -> Mitigation：发布步骤要求先应用新资源，再确认流量切换，最后删除旧资源；不宣称自动无损迁移。
- [Risk] Prometheus 历史序列因 `service` label 改名断裂 -> Mitigation：Grafana 和 alert 仅查询新 label，历史查询通过时间窗口和发布记录解释，不保留兼容 PromQL。
- [Risk] 文档、OpenAPI 示例或 Grafana 副本遗漏旧名 -> Mitigation：使用全仓 `rg` 检查旧复数和 `user-services` 二进制残留，并运行 `make user-service-openapi-generate`、`make compose-dashboard-check`。

## Migration Plan

实施顺序：

1. 更新 Go 入口和默认配置：CLI root、默认 `app.name`、JWT issuer、测试 fixture、Redis key prefix 期望和 metrics/log/tracing 断言。
2. 更新 Docker、CI 和 Compose：镜像名、二进制输出、ENTRYPOINT、healthcheck、Prometheus scrape label、real metrics load 脚本。
3. 更新 Kubernetes 和 Helm：目录、chart name、helper name、resource name、container name、image repository、Secret/ConfigMap/ServiceAccount/Job 引用和 README。
4. 更新 Prometheus/Grafana、docs、OpenAPI 生成物和 OpenSpec 主规格引用。
5. 执行命名残留检查、相关 Go 测试、OpenAPI 生成、dashboard 检查、architecture lint、暂存预期变更后执行 `make lint` 和 `make verify`。

回滚方式：回滚本次代码和部署资产提交，并重新部署旧镜像和旧 Kubernetes/Helm 资源。由于不保留兼容，回滚后新 issuer token、新 Redis key prefix 和新 Prometheus label 产生的数据不会自动迁回旧名。

验证方式：

- `rg -n 'aegiscore-user-services|user-services'` 只允许在 change 说明或归档历史中出现，不允许在当前代码、部署、文档和主规格中残留旧运行时契约。
- `go test ./cmd ./internal/bootstrap ./internal/providers ./internal/features/auth/... ./internal/features/permission/...`
- `make user-service-openapi-generate`
- `make compose-dashboard-check`
- `make user-service-architecture-lint`
- 暂存本次预期变更后运行 `make lint` 和 `make verify`。

## Open Questions

无。
