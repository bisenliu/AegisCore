## Why

当前交付规格和部署资产仍提供通过 Makefile、脚本、Compose、Kubernetes Job、Helm Job 或 CI/CD release job 连接数据库并自动执行 `atlas migrate apply` 的入口，这与目标流程中“SQL 进 Git 后由 DBA 工单或发布平台人工/受控执行”的治理要求不一致。

本变更收敛数据库 migration 生命周期：继续保留 Ent schema、Atlas diff、Atlas validate/hash 和 `atlas.sum`，但移除仓库内直接应用数据库变更的自动执行入口，降低发布资产误触生产数据库的风险。

## What Changes

- **BREAKING**: 移除根 `Makefile` 的 `user-service-migrate-apply` 目标和 `user-service/Makefile` 的 `migrate-apply` 目标，不再支持通过 `DATABASE_URL` 执行仓库内 apply 命令。
- **BREAKING**: 移除或改写 `user-service/scripts/migrate-apply.sh`，确保仓库不再提供自动连接数据库执行 `atlas migrate apply` 的脚本入口。
- **BREAKING**: 移除或改写 Compose、Kubernetes、Helm 中自动执行 `atlas migrate apply` 的 migration Job 资产和说明；保留部署资产对迁移顺序、RBAC seed 和 HTTP rollout 的受控发布说明。
- 调整交付文档，将数据库流程明确为：Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或发布平台人工/受控执行。
- 保留 Atlas 本身、`migrate diff`、`migrate validate`、`atlas.sum`、Atlas 配置和 SQL migration 目录。
- 在 baseline SQL migration 文件最前面添加 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`，并将 `users.nickname` 的 trigram 搜索索引调整为 GIN + `gin_trgm_ops`，确保扩展创建早于索引使用。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: 修改数据库 migration 和部署发布要求，从仓库内自动 apply / migration Job 执行改为只生成、校验和提交 SQL，由 DBA 工单或受控发布平台执行。

## Impact

- 影响根 `Makefile`、`user-service/Makefile`、`user-service/scripts/migrate-apply.sh` 和相关帮助文案。
- 影响 `deployments/compose/`、`deployments/k8s/user-services/`、`deployments/helm/aegiscore-user-services/` 中 migration Job 资产、values、README 和发布说明。
- 影响 `README.md`、`user-service/README.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md`、`AGENTS.md` 中关于 migration apply、migration Job 和发布顺序的说明。
- 影响 `openspec/specs/delivery-operations/spec.md` 的稳定要求，需要通过本 change 的 delta spec 更新。
- 影响 `user-service/migrations/20260701095702_latest_schema.sql` 和 `atlas.sum`，需要保留 SQL 可审查、可校验并明确 `pg_trgm` 扩展权限由 DBA 或受控发布流程处理。
