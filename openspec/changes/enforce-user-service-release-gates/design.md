## 受影响路径

- `.github/workflows/user-service-release.yml`
- `deployments/helm/aegiscore-user-service/values.yaml`
- `deployments/helm/aegiscore-user-service/templates/rbac-seed-job.yaml`
- `deployments/k8s/user-service/kustomization.yaml`
- `user-service/scripts/architecture/lint.sh`
- `deployments/helm/README.md`
- `deployments/helm/aegiscore-user-service/README.md`
- `deployments/k8s/user-service/README.md`

## 关键决策

- Helm 默认 `rbacSeedJob.enabled=false`，生产最终 runtime manifest 不包含 seed Job。需要执行 seed 时由 CI/CD 显式开启并单独渲染。
- seed Job 名称通过 `rbacSeedJob.nameSuffix` 注入 `rbac-seed-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}`，避免 upgrade 复用固定 Job 名称触发不可变 Pod template 更新失败。
- release workflow 仍先构建、扫描和推广同一镜像 digest，再生成两个 manifest：`aegiscore-user-service-rbac-seed.yaml` 与 `aegiscore-user-service-runtime.yaml`。
- 可选部署 job 通过 `migration_confirmation=migration-confirmed:<commit-sha>` 作为机器可校验输入；确认失败时不触碰集群工作负载。
- 部署阶段先 `kubectl apply` seed manifest 并等待 Job 完成，再 apply runtime manifest 并 `kubectl rollout status`。任一步骤失败会让发布阶段退出，后续 Deployment apply 不执行。

## 备选方案

- 使用 Helm hook 管理 seed Job：拒绝。Helm hook 仍会把顺序隐藏在 Helm 行为中，且失败诊断、GitOps 审查和最终 runtime manifest 边界不够明确。
- 在 Deployment initContainer 中执行 seed：拒绝。多副本会重复执行，且把运维前置条件耦合到运行时副本启动。
- 保留 Kustomize 默认聚合 seed Job：拒绝。默认 `kubectl apply -k` 仍可能绕过分阶段门禁。

## 风险与回滚

- 如果现有操作者依赖默认 Helm 渲染包含 seed Job，需要改为使用 release artifact 中的独立 seed manifest，或显式设置 `rbacSeedJob.enabled=true`。
- 若部署 job 配置错误，可回退到仅下载 artifacts 后按文档分阶段执行；runtime manifest 默认不含 seed Job，回滚 Deployment 不会隐式重跑 seed。

## 验证方式

- `make user-service-architecture-lint`
- `helm lint deployments/helm/aegiscore-user-service --set-string image.ref=aegiscore-user-service:sha-0000000000000000000000000000000000000000`
- `helm template` 分别验证 seed manifest 包含唯一 Job、runtime manifest 不包含 Job。
- `kubectl kustomize deployments/k8s/user-service` 验证默认聚合不包含 RBAC seed Job。
