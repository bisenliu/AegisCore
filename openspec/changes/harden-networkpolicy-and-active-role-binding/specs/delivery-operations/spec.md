## MODIFIED Requirements

### Requirement: 用户服务 Kubernetes 生产清单

系统 MUST 为 user-service 提供云厂商无关的 Kubernetes 生产清单，并覆盖运行副本、服务发现、配置引用、Secret 引用、安全上下文、资源配额、探针、滚动更新、PDB、HPA 和 NetworkPolicy。user-service NetworkPolicy MUST 默认使用显式来源与目的地约束，且准入标签 MUST 由 admission policy 或等价集群准入控制限制使用范围。

#### Scenario: 渲染生产清单

- **WHEN** 协作者查看或应用 `deployments/k8s/user-services/` 下的清单
- **THEN** 清单 MUST 包含 user-service 的 `Deployment`、`Service`、配置资源、Secret 引用、`ServiceAccount`、必要 RBAC、`PodDisruptionBudget`、`HorizontalPodAutoscaler` 和 `NetworkPolicy`

#### Scenario: HTTP 副本探针

- **WHEN** user-service Pod 由 Kubernetes Deployment 启动
- **THEN** liveness probe MUST 指向 `GET /livez`，readiness probe MUST 指向 `GET /readyz`，startup probe MUST 指向 `GET /startupz`

#### Scenario: 运行副本不执行 migration

- **WHEN** user-service Deployment 启动普通 HTTP 副本
- **THEN** Pod 环境变量 MUST NOT 设置 `RUN_MIGRATIONS=true`，副本 MUST 只启动 HTTP 服务

#### Scenario: Pod 安全和资源边界

- **WHEN** user-service Pod 被调度
- **THEN** Pod 和容器 securityContext MUST 默认使用非 root、只读根文件系统、禁止特权升级和收敛 Linux capabilities，并且容器 MUST 设置 CPU 与内存 requests/limits

#### Scenario: NetworkPolicy 入站来源约束

- **WHEN** user-service 原生 Kubernetes 生产清单声明 NetworkPolicy ingress
- **THEN** ingress 来源 MUST 使用明确的 `namespaceSelector` 与 `podSelector` 组合约束允许访问 user-service HTTP 端口的上游
- **AND** ingress MUST NOT 使用 `namespaceSelector: {}` 配合单个 Pod 标签作为生产默认来源约束

#### Scenario: NetworkPolicy 出站目的地约束

- **WHEN** user-service 原生 Kubernetes 生产清单声明访问 PostgreSQL、Redis 或 OTLP Collector 的 NetworkPolicy egress
- **THEN** 每类业务依赖 egress 规则 MUST 包含 `to` 目的地约束
- **AND** 目的地 MUST 使用目标 namespace、目标 Pod 标签或精确 `ipBlock` 约束实际 PostgreSQL、Redis 或 OTLP Collector
- **AND** egress MUST NOT 仅按 `5432`、`6379`、`4317` 或 `4318` 端口对任意目的地址放行

#### Scenario: 准入标签治理

- **WHEN** 集群使用标签表达允许访问 user-service 的上游身份
- **THEN** 部署资产 MUST 提供 admission policy 或等价准入治理说明，限制未授权 namespace 或 workload 自行设置该准入标签
- **AND** 未授权租户 MUST NOT 能通过自行添加准入标签获得访问 user-service 的 NetworkPolicy 入站许可

### Requirement: 用户服务 Helm chart

系统 MUST 为 `aegiscore-user-services` 提供 Helm chart，用 values 模板化 Kubernetes 交付能力，并保持默认值不包含真实生产 Secret。chart MUST 支持 RBAC seed Job 和 HTTP Deployment 使用 user-service 镜像，且 MUST NOT 渲染或默认配置自动执行 `atlas migrate apply` 的 migration Job。chart 的默认 NetworkPolicy values MUST 表达显式来源与目的地安全基线。

#### Scenario: Helm chart 元数据和 values

- **WHEN** 协作者查看 `deployments/helm/aegiscore-user-services/`
- **THEN** chart MUST 包含 `Chart.yaml`、`values.yaml`、templates、README 和环境覆盖示例，并 MUST 暴露 image、service、config、Secret 引用、resources、probes、autoscaling、PDB、NetworkPolicy、RBAC seed Job 和 rollout 配置
- **AND** chart MUST NOT 暴露默认执行 `atlas migrate apply` 的 migration Job 配置

#### Scenario: Helm 渲染 Secret 引用

- **WHEN** 协作者执行 `helm template` 渲染 chart
- **THEN** 模板 MUST 通过 `existingSecret` 或等价 values 引用外部 Secret，不得默认渲染真实敏感值

#### Scenario: Helm 渲染发布作业

- **WHEN** Helm values 启用 RBAC seed Job
- **THEN** chart MUST 渲染 RBAC seed Job
- **AND** RBAC seed Job MUST 使用 user-service 发布镜像执行 `rbac seed`
- **AND** chart MUST NOT 渲染自动执行 `atlas migrate apply` 的 migration Job

#### Scenario: Helm Deployment 默认行为

- **WHEN** Helm chart 渲染 user-service Deployment
- **THEN** Deployment MUST 默认不设置 `RUN_MIGRATIONS=true`，并 MUST 渲染 `/livez`、`/readyz`、`/startupz` 探针和资源 requests/limits
- **AND** Deployment 使用的 user-service 镜像 MUST NOT 依赖 Atlas 二进制或 migration SQL 文件启动 HTTP 服务

#### Scenario: Helm 默认 NetworkPolicy 入站来源

- **WHEN** 协作者使用默认 values 渲染 `aegiscore-user-services` chart
- **THEN** 渲染出的 NetworkPolicy ingress MUST 使用明确的 namespace 与 Pod 选择器约束允许访问 user-service HTTP 端口的来源
- **AND** 默认 values MUST NOT 使用 `namespaceSelector: {}` 配合单个 Pod 标签作为入站来源约束

#### Scenario: Helm 默认 NetworkPolicy 出站目的地

- **WHEN** 协作者使用默认 values 渲染 `aegiscore-user-services` chart
- **THEN** 渲染出的 PostgreSQL、Redis 和 OTLP Collector egress 规则 MUST 分别包含 `to` 目的地约束
- **AND** 默认 values MUST NOT 对任意目的地址开放 `5432`、`6379`、`4317` 或 `4318`

#### Scenario: Helm 环境覆盖外部依赖

- **WHEN** 目标环境使用集群外 PostgreSQL、Redis 或 OTLP Collector
- **THEN** 环境 values MUST 使用精确 `ipBlock` 或等价明确目的地覆盖默认 egress 目的地
- **AND** 环境 values MUST NOT 通过删除 `to` 字段恢复任意目的端口放行
