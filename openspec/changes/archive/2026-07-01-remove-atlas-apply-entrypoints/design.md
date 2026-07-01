## Context

AegisCore 目前将 Atlas SQL migration 作为可审查工件提交到 Git，但交付入口仍包含多条会连接数据库并执行 `atlas migrate apply` 的路径：根 Makefile、user-service Makefile、`migrate-apply.sh`、Compose migration service、Kubernetes migration Job、Helm migration Job 以及相关文档。目标流程要求仓库只负责生成和校验 SQL，实际数据库变更由 DBA 工单或发布平台在受控环境中执行。

本变更横跨 `Makefile`、`user-service/Makefile`、`user-service/scripts/`、`deployments/`、`docs/` 和 `openspec/`。不改变 Go 业务代码、HTTP API、OpenAPI 生成物、RBAC 业务语义或运行时容器不得执行 migration 的边界。

## Goals / Non-Goals

**Goals:**

- 移除仓库内所有面向协作者或部署资产的 `atlas migrate apply` 自动执行入口。
- 保留 Atlas 作为 diff、validate、hash 校验工具，继续提交 SQL migration 和 `atlas.sum`。
- 将数据库迁移流程统一表述为 Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或受控发布平台执行。
- 改写 Compose、Kubernetes、Helm 和文档，使它们不再渲染或要求自动 migration Job，同时仍明确 RBAC seed 与 HTTP rollout 必须在数据库变更完成后执行。
- 在 baseline SQL 中显式前置 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`，并将 `users.nickname` 搜索索引调整为 GIN + `gin_trgm_ops`。

**Non-Goals:**

- 不移除 Atlas CLI 的开发使用方式、`migrate diff`、`migrate validate`、`atlas.hcl`、SQL migration 目录或 `atlas.sum`。
- 不引入应用运行时自动 migration、Ent `client.Schema.Create(ctx)` 或业务账号隐式修改 schema 的能力。
- 不设计 DBA 工单系统或发布平台的具体实现，只在仓库文档中声明输入、顺序和责任边界。
- 不改变 user/auth/role/permission 的业务功能、HTTP 契约或 OpenAPI 文档。

## Decisions

1. 移除 apply 命令，而不是保留为“仅本地可用”。

   理由：同一命令在本地、CI 或生产中语义一致性很重要。保留 `DATABASE_URL make user-service-migrate-apply` 会继续鼓励从仓库直接连接数据库，和受控执行目标冲突。

   备选方案：保留命令但增加警告或 dry-run 默认值。该方案仍会留下误用路径，也会让文档同时维护两套流程。

2. 改写或移除 migration Job 资产，而不是继续渲染禁用状态的 Job。

   理由：部署资产不应默认携带可直接执行数据库 apply 的 manifest 或 Helm values。Kubernetes 与 Helm 文档应改为描述“发布前确认 SQL 已由 DBA 或受控平台执行”，而不是提交可运行的 Atlas Job。

   备选方案：保留 Job 模板并默认 `enabled: false`。该方案仍会把受控执行职责表达为仓库内 Job 能力，和目标边界不一致。

3. 保留 migration 镜像或 Atlas 配置的处理以最小化为准。

   理由：用户明确要求不要移除 Atlas 本身。实现时可以删除自动 apply 入口和文档引用，但必须保留 diff/validate/hash 所需配置。若 migration Dockerfile 只服务于 apply Job，应删除或改写其用途说明，避免形成新的自动执行入口。

   备选方案：彻底删除所有 Atlas 相关部署资产。该方案会误删 diff/validate 所需能力，不符合要求。

4. `pg_trgm` 扩展放入首个 SQL migration 文件首行。

   理由：`users.nickname` 的 trigram GIN 索引依赖 `pg_trgm`，扩展创建必须早于索引创建。将扩展声明作为 SQL 工件的一部分，也便于 DBA 工单或发布平台识别权限要求。

   备选方案：只在文档中要求 DBA 预创建扩展。该方案容易导致 SQL 与实际依赖分离，校验和工单审查不直观。

## Risks / Trade-offs

- [Risk] 删除 apply 入口后，开发者无法一键对本地数据库应用 migration。→ Mitigation: 文档明确本地也应通过受控 SQL 执行方式处理，仓库保留 diff/validate/hash 保证 SQL 工件正确。
- [Risk] Compose、Kubernetes、Helm 移除 migration Job 后，发布顺序可能被误解为无需先执行数据库变更。→ Mitigation: README、values 和规格中明确 RBAC seed 与 HTTP rollout 的前置条件是目标环境已完成对应 SQL migration。
- [Risk] `CREATE EXTENSION IF NOT EXISTS pg_trgm;` 在生产库可能需要更高权限。→ Mitigation: SQL 和文档说明该语句需要纳入 DBA 工单；如业务账号无权限，由 DBA 作为前置动作执行。
- [Risk] 手动修改 baseline SQL 后 `atlas.sum` 失效。→ Mitigation: tasks 要求刷新并提交 `atlas.sum`，并运行 `make user-service-migrate-validate`。

## Migration Plan

1. 先更新 OpenSpec delta，明确新的 migration 与发布稳定行为。
2. 删除或改写 Makefile、脚本、Compose、Kubernetes、Helm 中的自动 apply 入口。
3. 更新 README、开发、测试、架构和 Agent 指南中的 migration apply 说明。
4. 修改 baseline SQL，前置 `pg_trgm` 扩展并调整 nickname trigram 索引，刷新 `atlas.sum`。
5. 运行 `make user-service-migrate-validate`、`make user-service-architecture-lint`，并对 Helm/Kubernetes/Compose 文档建议的渲染或 YAML 检查执行验证。

回滚策略：如需要恢复自动 apply 能力，必须通过新的 OpenSpec change 重新定义受控边界和部署资产；不能仅恢复旧 Makefile 目标或 Job 模板。

## Open Questions

- 无待决问题；实现时如发现某个 deployment 资产既服务 apply 又服务 validate，应优先保留 validate 所需部分，删除可连接数据库执行 apply 的路径。
