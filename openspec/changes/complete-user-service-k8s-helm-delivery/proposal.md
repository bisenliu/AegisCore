## Why

`deployments/k8s/user-services/` 和 `deployments/helm/aegiscore-user-services/` 当前只有占位 README，无法支撑 user-service 的云原生发布、容量保护、安全基线和发布顺序治理。

本次变更需要把占位边界升级为可审查、可模板化、可验证的 Kubernetes 与 Helm 交付资产，确保生产发布先执行 Atlas migration，再执行 RBAC seed，最后滚动 HTTP 副本。

## What Changes

- 新增 user-service 的 Kubernetes 原生清单，覆盖 Deployment、Service、ConfigMap、Secret 引用、ServiceAccount、必要 RBAC、独立 migration Job、RBAC seed Job、HPA、PDB、NetworkPolicy、Pod/Container securityContext、资源 requests/limits、探针和滚动更新策略。
- 新增 `aegiscore-user-services` Helm chart 元数据、默认 `values.yaml`、环境覆盖示例和 templates，暴露镜像、配置、Secret 引用、探针、资源、autoscaling、PDB、NetworkPolicy、migration Job、RBAC seed Job 和 rollout 策略配置。
- 更新 Kubernetes 与 Helm README，明确生产发布顺序、Secret 注入方式、验证命令、回滚注意事项和与观测资产的关系。
- 修改 `delivery-operations` 规格，增加 Kubernetes/Helm 生产交付资产必须满足的稳定行为。
- 不改变 user-service HTTP API、OpenAPI 契约、Ent schema 或业务运行时行为。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `delivery-operations`: 将 Kubernetes 和 Helm 从占位边界升级为生产可用交付资产，补充 migration、RBAC seed、rollout、安全、容量和验证要求。

## Impact

- 影响路径：`deployments/k8s/user-services/`、`deployments/helm/aegiscore-user-services/`、`openspec/specs/delivery-operations/spec.md` 对应 delta、相关 README 或验证说明。
- 部署影响：新增独立 migration Job 和 RBAC seed Job，生产发布顺序从文档约束变为部署资产可执行流程。
- 安全影响：Secret 只通过外部 Secret 或部署系统引用注入；Pod 默认启用非 root、只读根文件系统、能力收敛和 NetworkPolicy 最小连通边界。
- 可靠性影响：新增 probes、resources、HPA、PDB 和滚动更新策略，降低不可用 rollout 和资源争抢风险。
- 验证影响：需要增加 Helm template、YAML/schema lint、架构 lint 和必要的渲染检查；如果引入新验证工具，应在任务中明确安装或替代方案。
