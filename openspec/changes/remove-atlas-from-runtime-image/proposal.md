## Why

当前 user-service 运行时镜像从 `arigaio/atlas:latest` 复制 Atlas 二进制，即使普通 HTTP 副本默认不执行 migration，也会随服务镜像携带约百 MB 的迁移工具。实测当前镜像中 `/usr/local/bin/atlas` 约 `110MB`，这会放大镜像拉取、存储和发布成本，并把迁移工具暴露在不需要执行迁移的运行时容器内。

## What Changes

- 从 user-service 运行时镜像中移除 Atlas 二进制和 migration apply 脚本依赖，普通服务镜像只承载 HTTP 服务与 RBAC CLI 运行所需内容。
- 将 migration Job 调整为使用独立 Atlas/migration 镜像执行已提交的 `user-service/migrations/` SQL migration，并继续通过 Secret 或发布系统注入 `DATABASE_URL`。
- 更新 Compose、Kubernetes、Helm 和发布文档，使 migration Job 与 HTTP Deployment 镜像职责分离，RBAC seed Job 继续使用 user-service 发布镜像。
- 移除或废弃 `RUN_MIGRATIONS=true` 在普通服务镜像中执行 Atlas migration 的兼容路径，避免运行时镜像重新引入 Atlas。
- 明确镜像体积优化验证和 migration 发布验证要求，确保服务镜像变小且迁移发布顺序不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: 调整 user-service Docker 镜像职责、migration Job 镜像来源、Compose/Kubernetes/Helm 发布作业约束，以及容器启动 migration 兼容模式要求。

## Impact

- 影响 Docker 构建：`deployments/docker/user-service.Dockerfile` 不再复制 Atlas，可能新增或调整专用 migration 镜像构建方式。
- 影响脚本：`user-service/scripts/migrate-apply.sh` 与 `entrypoint.sh` 的适用边界需要调整，普通服务镜像不得依赖 Atlas。
- 影响 Compose：`deployments/compose/docker-compose.yml` 的 `user-service-migrate` one-shot 服务需要改用独立 Atlas/migration 镜像或专用构建目标。
- 影响 Kubernetes：`deployments/k8s/user-services/migration-job.yaml` 的 migration Job 镜像、command、volume 或镜像内容需要与独立迁移执行方式一致。
- 影响 Helm：`deployments/helm/aegiscore-user-services` 需要暴露 migration 镜像配置，并保持 RBAC seed Job 使用 user-service 镜像。
- 影响文档和规格：README、部署说明和 `delivery-operations` 主规格需要从“migration Job 使用当前发布镜像”改为“migration Job 使用独立 Atlas/migration 镜像”。
- 不影响 HTTP API、OpenAPI、数据库 schema、Ent 生成代码、业务权限语义或 Casbin 授权行为。
