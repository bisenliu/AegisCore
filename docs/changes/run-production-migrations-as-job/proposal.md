# Run production migrations as job

## What

将用户服务生产数据库迁移从默认容器启动路径中拆出，改为由独立 migration Job 或 CI/CD release job 显式执行。

本变更建议调整用户服务容器入口和部署约定：

- `user-service/scripts/entrypoint.sh` 默认不再执行 Atlas migration。
- 生产部署必须在启动或滚动更新用户服务副本前，先执行一次独立 migration Job。
- 本地 Compose 继续使用独立 `user-service-migrate` 服务在用户服务启动前执行迁移。
- `RUN_MIGRATIONS=true` 仅作为简单部署或兼容场景的显式开关，不作为生产默认行为。
- 文档补充生产发布顺序、失败处理、回滚边界和多副本滚动发布约束。

本变更只调整迁移执行策略、脚本默认值、部署资产和文档，不修改 Ent schema、Atlas SQL migration、数据库结构、HTTP API 或业务逻辑。

## Why

当前入口脚本在 `RUN_MIGRATIONS` 未设置时默认执行迁移。该模式对单实例或简单环境方便，但在生产多副本滚动发布时会带来三个问题：

- 启动耗时和迁移耗时耦合，影响 rollout 节奏、探针窗口和故障定位。
- 多副本同时启动时依赖 Atlas migration lock 串行化，虽然可避免并发写迁移表，但仍会让普通服务副本承担发布编排责任。
- 迁移失败和服务启动失败混在一起，CI/CD 无法在进入应用 rollout 前清晰阻断。

生产环境应把数据库 schema 迁移视为发布阶段中的显式步骤：迁移 Job 成功后再启动或滚动更新应用副本。这样可以让服务容器启动路径只负责 HTTP runtime，降低多副本发布耦合，并让迁移失败在应用副本变更前暴露。

## Scope

包括：

- 调整 `user-service/scripts/entrypoint.sh`：
  - 将默认值从 `RUN_MIGRATIONS=true` 改为 `RUN_MIGRATIONS=false`。
  - 保留 `RUN_MIGRATIONS=true` 时调用 `/app/user-service/scripts/migrate-apply.sh` 的兼容能力。
  - 更新中文注释，明确生产优先使用独立 migration Job。
- 更新 Docker/Compose 说明：
  - 确认 `deployments/compose/docker-compose.yml` 继续通过 `user-service-migrate` 服务执行迁移。
  - 确认 `user-service` 服务显式设置 `RUN_MIGRATIONS=false`。
  - 文档说明 Compose 的迁移服务是本地 release job 模拟，不代表生产应用容器启动时迁移。
- 新增或更新 Kubernetes/Helm 部署说明：
  - 明确生产应提供独立 Job 执行 `/app/user-service/scripts/migrate-apply.sh`。
  - 应用 Deployment 不应依赖入口脚本默认迁移。
  - Job 成功后再执行 RBAC seed 和服务 rollout。
- 更新 `docs/DEVELOPMENT.md`、`docs/ARCHITECTURE.md`、`docs/PRODUCT.md`、`docs/TESTING.md` 中关于“服务启动前执行迁移”的表述。
- 如已有部署模板或未来 Helm chart，预留 migration job 开关、镜像、环境变量和 secret 引用的边界说明。

不包括：

- 不生成新的 SQL migration。
- 不修改 `user-service/migrations/*.sql` 或 `atlas.sum`。
- 不修改 Ent schema 或生成 Ent 代码。
- 不修改业务 HTTP API、feature 代码、Fx provider 或数据库访问逻辑。
- 不实现云厂商特定 CI/CD、Argo CD hook、Helm hook 或 Kubernetes Secret 管理方案，除非后续实现阶段已有对应部署模板。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `entrypoint.sh` 在未设置 `RUN_MIGRATIONS` 时不会执行 migration。
- 设置 `RUN_MIGRATIONS=true` 时，入口脚本仍会先执行 `/app/user-service/scripts/migrate-apply.sh`，保持兼容。
- 本地 Compose 仍能通过独立 `user-service-migrate` 服务在用户服务启动前执行 migration，并且用户服务容器保持 `RUN_MIGRATIONS=false`。
- 生产部署文档明确要求在应用 rollout 前执行独立 migration Job 或 CI/CD release job。
- 文档中的发布顺序统一为：migration、RBAC seed、启动或滚动更新 HTTP server。
- 文档说明多副本滚动发布不应依赖普通服务容器启动时迁移，Atlas lock 只作为兼容场景保护，不作为生产编排手段。
- 迁移失败应阻断应用 rollout；应用容器启动失败不应自动重试 schema migration，除非显式启用兼容开关。
- `make migrate-validate` 仍通过。
- 不修改历史 SQL、Ent schema、Ent generated code、HTTP API 或业务逻辑。
