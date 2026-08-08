## Why

当前 user-service Helm release、原生 Kustomize 聚合和 release workflow 只在文档中描述 `migration -> RBAC seed -> Deployment rollout` 顺序。直接应用合并后的 manifest 时，RBAC seed Job 与 HTTP Deployment 可能同时进入集群，导致新版副本在数据库结构或 RBAC 基线未确认前启动；固定名称 Job 在后续升级中还会遇到 Pod template 不可变约束。

## What Changes

- 将 Helm 默认发布产物改为 runtime-only，RBAC seed Job 仅在显式开启时渲染。
- release workflow 分别生成 release 唯一 RBAC seed manifest 和最终 runtime manifest，并记录同一镜像 digest。
- release workflow 增加可选受控部署阶段：校验 migration 确认、应用并等待 seed Job 成功、再应用 runtime manifest 并等待 Deployment rollout。
- 原生 Kustomize 默认聚合移除 RBAC seed Job，保留 Job 文件作为分阶段发布入口。
- 更新架构 lint，防止 Helm 默认重新启用 seed Job 或 Kustomize 默认聚合 seed Job。

## Capabilities

- `delivery-operations`

## Impact

- 影响 `.github/workflows/user-service-release.yml`、`deployments/helm/aegiscore-user-service/`、`deployments/k8s/user-service/` 和架构 lint 脚本。
- 不影响 Go 业务代码、HTTP API、数据库 schema、SQL migration 或 OpenAPI 生成物。
- 发布流程新增可执行门禁；未提供 migration 确认或 seed 失败时，不更新 Deployment。
