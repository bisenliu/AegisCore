## 1. Kubernetes 原生清单

- [x] 1.1 在 `deployments/k8s/user-services/` 新增命名、label 和配置基线，包含 `kustomization.yaml` 或等价入口，确保所有资源可一次性发现和应用。
- [x] 1.2 新增 user-service `Deployment` 和 `Service`，设置 `/livez`、`/readyz`、`/startupz` 探针、rollingUpdate、terminationGracePeriod、resources、Pod/Container securityContext，并确认不设置 `RUN_MIGRATIONS=true`。
- [x] 1.3 新增 `ConfigMap` 和 Secret 引用边界，覆盖 HTTP、日志、metrics、tracing、PostgreSQL、Redis、JWT 和 `DATABASE_URL` 所需环境变量，不提交真实敏感值。
- [x] 1.4 新增 migration `Job`，使用当前发布镜像执行 `/app/user-service/scripts/migrate-apply.sh`，通过 Secret 引用注入 `DATABASE_URL`，设置合理的 restartPolicy、backoffLimit 和清理策略。
- [x] 1.5 新增 RBAC seed `Job`，使用当前发布镜像执行 `rbac seed --reactivate-system --sync-system-bindings`，复用服务配置和 Secret 引用，并说明必须在 migration 成功后执行。
- [x] 1.6 新增 `ServiceAccount`、必要 RBAC、`PodDisruptionBudget`、`HorizontalPodAutoscaler` 和 `NetworkPolicy`，保持默认资源云厂商无关。

## 2. Helm chart

- [x] 2.1 在 `deployments/helm/aegiscore-user-services/` 新增 `Chart.yaml`、`values.yaml`、`values-local.yaml` 或等价示例和 `templates/` 结构。
- [x] 2.2 实现 Deployment、Service、ConfigMap、ServiceAccount、RBAC、PDB、HPA、NetworkPolicy 模板，并保持探针、resources、securityContext 和 rollout 默认值与原生 YAML 语义一致。
- [x] 2.3 实现 migration Job 和 RBAC seed Job 模板，支持 values 启用开关、镜像复用、command 覆盖、Secret 引用、backoffLimit 和 ttlSecondsAfterFinished。
- [x] 2.4 实现 helper templates，统一 fullname、labels、selectorLabels、serviceAccountName、image、env 和 Secret key 引用，避免模板间命名 drift。
- [x] 2.5 确认 Helm 默认 values 不渲染真实 Secret，不默认设置 `RUN_MIGRATIONS=true`，并能通过 values 引用既有 Secret。

## 3. 文档和发布流程

- [x] 3.1 更新 `deployments/k8s/README.md` 和 `deployments/k8s/user-services/README.md`，说明资源清单、Secret 准备、migration Job、RBAC seed Job、Deployment rollout、失败诊断和回滚步骤。
- [x] 3.2 更新 `deployments/helm/README.md` 和 `deployments/helm/aegiscore-user-services/README.md`，说明 chart values、Secret 引用、发布顺序、渲染检查、回滚边界和与观测资产的关系。
- [x] 3.3 如实现中新增固定验证入口，更新根 `Makefile` 或部署 README，保持服务私有目标带 `user-service-` 前缀。
- [x] 3.4 检查 OpenSpec artifacts 与实现文档是否一致，确保本 change 不引入 Go API、OpenAPI、Ent schema 或 migration 变更。

## 4. 验证

- [x] 4.1 执行 `helm lint deployments/helm/aegiscore-user-services`。
- [x] 4.2 执行 `helm template aegiscore-user-services deployments/helm/aegiscore-user-services --values deployments/helm/aegiscore-user-services/values.yaml`，检查输出包含 migration Job、RBAC seed Job、Deployment probes、resources、PDB、HPA 和 NetworkPolicy。
- [x] 4.3 使用可用工具校验原生 YAML，例如 `kubectl apply --dry-run=client -k deployments/k8s/user-services`、`kubectl apply --dry-run=client -f deployments/k8s/user-services` 或 README 中记录的等价命令。
- [x] 4.4 执行 `openspec validate complete-user-service-k8s-helm-delivery`。
- [x] 4.5 执行 `make user-service-architecture-lint`，确认 OPSX 文档和架构边界检查通过。
- [x] 4.6 执行 `git diff --check`，确认 YAML、Helm templates 和 Markdown 没有空白错误。
