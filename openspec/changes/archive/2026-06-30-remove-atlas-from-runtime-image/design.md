## Context

user-service 当前使用单一运行时镜像同时承载 HTTP 服务、RBAC CLI 和 Atlas migration 执行能力。`deployments/docker/user-service.Dockerfile` 从 `arigaio/atlas:latest` 复制 `/atlas` 到 `/usr/local/bin/atlas`，`user-service/scripts/migrate-apply.sh` 直接调用 `atlas migrate apply`，Compose、Kubernetes 和 Helm 的 migration Job 当前也复用 user-service 镜像执行该脚本。

普通 HTTP Deployment 默认不设置 `RUN_MIGRATIONS=true`，生产发布也已经要求先执行独立 migration Job，再执行 RBAC seed Job，最后滚动 HTTP 副本。因此 Atlas 对 HTTP 运行时不是必需依赖。保留 Atlas 会增加镜像体积、拉取时间和运行时攻击面，也使服务镜像职责混合。

受影响路径包括 `deployments/docker/`、`deployments/compose/`、`deployments/k8s/user-services/`、`deployments/helm/aegiscore-user-services/`、`user-service/scripts/`、部署 README、`docs/` 和 `openspec/specs/delivery-operations/`。本变更不涉及 `common`、业务 feature、HTTP API、OpenAPI、Ent schema 或数据库 SQL migration 内容。

## Goals / Non-Goals

**Goals:**

- user-service HTTP 运行时镜像不再包含 Atlas 二进制。
- migration Job 使用独立 Atlas/migration 镜像执行已提交的 `user-service/migrations/`。
- Compose、Kubernetes 和 Helm 继续保证 migration 先于 RBAC seed，RBAC seed 先于 HTTP 服务启动或 rollout。
- Helm values 能独立配置 migration 镜像，不强制与 user-service 镜像相同。
- 文档和 OpenSpec 规格准确描述新的镜像职责边界和验证方式。
- 验证包含镜像构建、migration 校验、Compose/Kubernetes/Helm 渲染或静态检查、架构文档检查。

**Non-Goals:**

- 不调整数据库 schema、Ent schema、migration SQL 或 `atlas.sum`。
- 不改变 HTTP API、认证、RBAC、用户业务行为或 OpenAPI 生成物。
- 不引入运行时自动 schema create 或绕过 Atlas SQL migration 的机制。
- 不把 Atlas migration 能力放入 `common`、feature 包、`internal/shared` 或业务运行时 Go 代码。
- 不把 RBAC seed Job 改成 Atlas 镜像；RBAC seed 仍依赖 user-service CLI。

## Decisions

1. user-service 运行时镜像移除 Atlas。

   理由：HTTP 服务和 RBAC CLI 不需要 Atlas。移除 `COPY --from=atlas /atlas /usr/local/bin/atlas` 可直接减少约百 MB 镜像内容，并避免普通服务容器具备数据库迁移工具。

   备选方案：继续保留 Atlas，但仅通过文档要求不在 HTTP 副本使用。该方案不能解决镜像体积和运行时工具暴露问题，因此不采用。

2. migration Job 使用独立 migration 镜像，而不是 user-service 发布镜像。

   理由：migration Job 的职责是运行 Atlas 和已提交 SQL migration，所需内容与 HTTP 服务镜像不同。专用 migration 镜像可以基于 `arigaio/atlas`，复制 `user-service/migrations/`，并用发布 tag 与服务镜像保持版本对齐。

   备选方案：Kubernetes 使用官方 `arigaio/atlas` 镜像并通过 ConfigMap 挂载 migration 文件。该方案可以移除服务镜像中的 Atlas，但 SQL 文件数量和大小受 ConfigMap 管理约束，且更容易产生镜像版本与 migration 内容漂移。Compose 和 CI/CD 也需要额外挂载逻辑。因此优先采用随发布构建的专用 migration 镜像。

3. `RUN_MIGRATIONS=true` 兼容模式从普通服务镜像职责中移除或显式废弃。

   理由：该模式要求服务镜像内存在 Atlas，与“运行时镜像不包含 Atlas”的目标冲突。生产发布已采用独立 migration Job；简单部署也可以运行专用 migration 镜像完成迁移后再启动服务。

   备选方案：保留 `RUN_MIGRATIONS=true` 并在入口脚本中动态下载 Atlas。该方案引入网络依赖、供应链风险和启动不确定性，不符合可审查发布流程，因此不采用。

4. Helm 为 migration Job 暴露独立镜像 values。

   理由：Helm chart 需要同时支持 user-service HTTP/RBAC seed 镜像和 migration 镜像。`migrationJob.image` 应能设置 repository、tag、pullPolicy，并默认可继承 chart appVersion 或显式使用发布系统传入的 migration image tag。

   备选方案：要求 migration 镜像名称由外部直接替换模板。该方案降低 chart 自描述性，也不利于 `helm template` 静态验证，因此不采用。

5. 验证以部署资产和文档为主，不运行数据库破坏性迁移。

   理由：本变更不改变 SQL migration 内容。实现时应运行 `make user-service-migrate-validate` 校验 migration 目录，使用 Docker build 和容器内文件检查确认服务镜像不含 Atlas，并通过 Compose config、Helm lint/template、Kubernetes dry-run 或 YAML 检查验证发布资产。

## Risks / Trade-offs

- [Risk] migration 镜像 tag 与 user-service 镜像 tag 不一致会导致发布版本漂移。→ Mitigation: Compose、Kubernetes、Helm 文档和 values 要求 migration 镜像与服务镜像使用同一 release tag 或同一发布流水线产物。
- [Risk] 移除 `RUN_MIGRATIONS=true` 后，依赖服务容器启动时迁移的简单部署流程会失败。→ Mitigation: 文档提供先运行 migration 镜像、再启动服务镜像的替代流程，并在 entrypoint 注释中明确普通服务镜像不执行 migration。
- [Risk] 专用 migration 镜像仍需要携带 Atlas，因此总体发布产物数量增加。→ Mitigation: 只让一次性 Job 拉取 migration 镜像，HTTP 副本不再重复拉取 Atlas 内容，生产高副本场景收益更大。
- [Risk] Helm 模板迁移镜像配置不完整会导致 migration Job 渲染错误。→ Mitigation: tasks 中要求执行 `helm lint` 和 `helm template`，并检查 migration Job image、command、Secret 引用和发布顺序。
- [Risk] Dockerfile 层仍可能因后续 `chmod` 复制大二进制造成镜像膨胀。→ Mitigation: 实现时应避免在复制服务二进制后用单独层修改其元数据，优先通过 `COPY --chmod` 或构建阶段权限控制减少重复层。

## Migration Plan

1. 新增或调整专用 migration Dockerfile，使其基于 `arigaio/atlas` 并复制 `user-service/migrations/`。
2. 修改 user-service 运行时 Dockerfile，移除 Atlas stage、Atlas copy 和运行时 migration apply 依赖。
3. 修改 Compose 的 `user-service-migrate`，使用 migration 镜像执行 `atlas migrate apply`，并保持 `rbac-seed` 和 `user-service` 的依赖顺序。
4. 修改 Kubernetes migration Job，使用 migration 镜像和 Atlas command，继续通过 Secret 注入 `DATABASE_URL`。
5. 修改 Helm values 和模板，支持独立 migration image，并保持 RBAC seed Job 使用 user-service image。
6. 更新 README、架构/开发/测试或部署说明中关于 migration Job 使用镜像和 `RUN_MIGRATIONS` 的描述。
7. 运行验证命令并记录镜像大小对比。

回滚方式：如 migration 镜像发布失败，可临时回滚到上一版 user-service 镜像和上一版部署资产，让 migration Job 继续使用包含 Atlas 的服务镜像；回滚时不得混用新版“无 Atlas 服务镜像”和旧版“使用服务镜像执行 migration”的 Job 模板。

## Open Questions

无。实施时以“专用 migration 镜像随 release 构建并与 user-service 镜像 tag 对齐”为默认方案。
