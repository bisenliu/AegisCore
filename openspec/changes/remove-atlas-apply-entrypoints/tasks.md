## 1. 命令和脚本入口

- [x] 1.1 删除根 `Makefile` 的 `user-service-migrate-apply` 目标和相关 help 文案，保留 `user-service-migrate-diff`、`user-service-migrate-validate` 和生成入口。
- [x] 1.2 删除或改写 `user-service/Makefile` 的 `migrate-apply` 目标和相关 help 文案，确保 `make -C user-service help` 不再暴露 apply 命令。
- [x] 1.3 删除或改写 `user-service/scripts/migrate-apply.sh`，确保仓库不再提供通过 `DATABASE_URL` 连接数据库执行 `atlas migrate apply` 的脚本入口。
- [x] 1.4 全仓搜索 `migrate-apply`、`user-service-migrate-apply`、`atlas migrate apply`、`migrate apply` 和 `RUN_MIGRATIONS`，记录需要改写的剩余引用并确认没有新的 apply 入口残留。

## 2. 部署资产调整

- [x] 2.1 改写 `deployments/compose/docker-compose.yml`，移除自动执行 Atlas apply 的 `user-service-migrate` service，并调整 `rbac-seed`、`user-service` 依赖为数据库 SQL 已受控执行后的启动关系。
- [x] 2.2 更新 `deployments/compose/README.md`，将本地 Compose 说明改为要求先确认 SQL migration 已由 DBA 工单或受控发布平台执行，不再描述 migration service 自动 apply。
- [x] 2.3 删除或改写 `deployments/k8s/user-services/migration-job.yaml`，确保原生 Kubernetes 清单不再提供自动执行 `atlas migrate apply` 的 Job。
- [x] 2.4 更新 `deployments/k8s/user-services/README.md` 和 `deployments/k8s/README.md`，将发布顺序改为先确认 SQL migration 受控执行完成，再执行 RBAC seed Job，最后 rollout HTTP Deployment。
- [x] 2.5 改写 `deployments/helm/aegiscore-user-services/templates/migration-job.yaml`、`values.yaml`、`values-local.yaml` 和 chart README，确保 Helm 不再渲染或默认配置自动执行 `atlas migrate apply` 的 migration Job。
- [x] 2.6 更新 `deployments/helm/README.md`，移除检查 migration Job command 的说明，改为检查 Helm 渲染结果不包含自动 apply Job，并保留 RBAC seed 与 Deployment 字段检查。
- [x] 2.7 处理 `deployments/docker/user-service-migration.Dockerfile`：如其只服务自动 apply Job，则删除或改写为非自动执行说明；不得留下默认 `CMD` 执行 `atlas migrate apply`。

## 3. SQL migration 和 Atlas 校验

- [x] 3.1 在 `user-service/migrations/20260701095702_latest_schema.sql` 文件首行添加 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`，确保早于所有依赖 `pg_trgm` 的索引。
- [x] 3.2 将 `users.nickname` trigram 搜索索引调整为 GIN + `gin_trgm_ops`，并确认 SQL 文件中没有先创建索引后创建扩展的顺序问题。
- [x] 3.3 刷新并提交 `user-service/migrations/atlas.sum`，确保手动 SQL 调整后的 hash 与 migration 目录一致。
- [x] 3.4 运行 `make user-service-migrate-validate`，验证 SQL migration 和 `atlas.sum` 通过 Atlas 校验。

## 4. 文档和规格同步

- [x] 4.1 更新 `README.md`、`user-service/README.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md` 和 `AGENTS.md`，移除 `DATABASE_URL='<postgres-url>' make user-service-migrate-apply` 与自动 migration Job 说明。
- [x] 4.2 将所有数据库 migration 文档统一为 Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或受控发布平台执行。
- [x] 4.3 在相关文档中说明 `CREATE EXTENSION IF NOT EXISTS pg_trgm;` 生产执行可能需要 DBA 权限；若业务账号无权限，应作为 DBA 前置动作纳入工单。
- [x] 4.4 确认 `openspec/changes/remove-atlas-apply-entrypoints/specs/delivery-operations/spec.md` 与最终实现一致，必要时补充 delta 场景。

## 5. 验证和 drift 检查

- [x] 5.1 运行 `make user-service-architecture-lint`，验证 OPSX 文档、OpenSpec 中文约束和架构边界。
- [x] 5.2 对 Compose、Kubernetes 和 Helm 资产执行适用的 YAML/schema 或渲染检查，确认不再包含自动执行 `atlas migrate apply` 的 Job/service/command。
- [x] 5.3 运行 `make lint` 和相关 Go 测试；若本变更不涉及 Go 代码，仍需确认 Makefile 和脚本变更未破坏交付入口。
- [x] 5.4 运行 `make verify` 或记录无法运行的环境原因；完成后检查 `git diff --exit-code` 或人工审查生成物 diff，确保只包含预期变更。
