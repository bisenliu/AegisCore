# Tasks

## Implementation

- [x] 更新 `user-service/scripts/entrypoint.sh`，将 `RUN_MIGRATIONS` 默认值从 `true` 改为 `false`。
- [x] 更新 `user-service/scripts/entrypoint.sh` 中文注释，说明入口脚本迁移只在 `RUN_MIGRATIONS=true` 时执行，生产优先使用独立 migration Job。
- [x] 检查 `user-service/scripts/migrate-apply.sh`，确认它仍适合作为独立 Job command，且只依赖 `DATABASE_URL`、Atlas CLI 和镜像内 migration assets。
- [x] 检查 `deployments/compose/docker-compose.yml`，确认 `user-service-migrate` 仍以 one-shot service 执行 `/app/user-service/scripts/migrate-apply.sh`。
- [x] 检查 `deployments/compose/docker-compose.yml`，确认 `user-service` 服务显式设置 `RUN_MIGRATIONS=false`。
- [x] 更新 `deployments/compose/README.md`，说明 Compose 的 `user-service-migrate` 是本地 migration job 模拟，并且用户服务容器本身不默认迁移。
- [x] 更新 `deployments/k8s/README.md` 和 `deployments/k8s/user-services/README.md`，说明未来生产 Kubernetes 清单必须提供独立 migration Job，Deployment 不应依赖入口脚本默认迁移。
- [x] 更新 `deployments/helm/README.md` 和 `deployments/helm/aegiscore-user-services/README.md`，说明未来 chart 应提供 migration job 配置边界，且 Deployment 默认不启用 startup migration。
- [x] 更新 `docs/ARCHITECTURE.md` 数据库迁移章节，将生产迁移规则表述为独立 CI/CD release job 或 migration Job 在 HTTP runtime rollout 前执行。
- [x] 更新 `docs/DEVELOPMENT.md` 发布和迁移说明，明确推荐顺序为 migration job、RBAC seed、HTTP server rollout，并将 entrypoint migration 降级为显式兼容选项。
- [x] 更新 `docs/TESTING.md` 中涉及 migration 验证和部署前校验的说明，确认测试仍通过 Atlas SQL migration 初始化 schema。
- [x] 更新 `docs/PRODUCT.md` 中“服务启动前执行”相关表述，改为发布流程显式迁移。
- [x] 如 `AGENTS.md` 中迁移策略描述需要同步，更新为生产默认独立 migration Job，并保留 `RUN_MIGRATIONS=true` 兼容说明。
- [x] 使用 `rg` 检查 `RUN_MIGRATIONS`、`entrypoint`、`容器入口`、`migration job`、`服务启动前执行迁移` 等表述，确认文档语义一致。

## Verification

- [x] 运行 `sh -n user-service/scripts/entrypoint.sh`。
- [x] 运行 `make migrate-validate`。
- [x] 运行 `docker compose -f deployments/compose/docker-compose.yml config`，确认 Compose 配置语法通过。
- [x] 如本地 Docker 依赖可用，运行 Compose migration/seed/service 启动路径，确认 `user-service-migrate` 成功后 `rbac-seed` 和 `user-service` 才启动。
- [x] 手工检查 `entrypoint.sh`，确认未设置 `RUN_MIGRATIONS` 时不会调用 `migrate-apply.sh`。
- [x] 手工检查 `entrypoint.sh`，确认设置 `RUN_MIGRATIONS=true` 时仍会调用 `migrate-apply.sh`。
- [x] 检查 `git diff -- user-service/scripts deployments docs AGENTS.md`，确认没有修改历史 SQL、Ent schema、Ent generated code、HTTP API 或业务逻辑。

## Review Notes

- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
- [x] 确认本变更不引入真实数据库凭据、Secret 样例明文或云厂商特定资源。
- [x] 确认生产迁移失败会阻断 rollout 的规则在文档中清晰可见。
- [x] 确认 Compose 本地体验仍然是一条命令启动依赖、迁移、RBAC seed 和用户服务。
- [x] 确认 `RUN_MIGRATIONS=true` 被描述为兼容/简单部署开关，而不是生产推荐路径。
