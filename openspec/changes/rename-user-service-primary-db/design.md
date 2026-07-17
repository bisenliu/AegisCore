## Context

当前 user-service 配置通过 `resources.postgres.user_db` 声明必需 PostgreSQL 资源，并在 Fx 中以 `name:"user_db"` 注入 `*sql.DB` 与 `*ent.Client`。该资源实际是服务主读写库，不是单一用户表或用户资料能力专用库。

相关路径包括 `user-service/internal/resources`、`user-service/internal/config`、`user-service/internal/providers`、feature PostgreSQL adapter、`cmd/rbac_dependencies.go`、`deployments`、`docs` 和观测资产。

## Goals / Non-Goals

**Goals:**

- 使用 `primary_db` 表达 user-service 主 PostgreSQL 资源职责。
- 保持 `resources.postgres` 具名资源结构不变。
- 同步所有严格配置路径、环境变量、metrics label、health 示例和文档引用。

**Non-Goals:**

- 不改数据库 schema、migration 或真实 `db_name`。
- 不引入多库、读写分离、审计库或支付/订单数据库配置。
- 不移除 `resources` 命名空间。

## Decisions

### Decision: 保留 `resources.postgres`，仅改资源名

选择将 `resources.postgres.user_db` 改为 `resources.postgres.primary_db`。备选方案是移除 `resources` 或将多个 `db_*` 放在同一 PostgreSQL 配置下，但这些方案会削弱外部资源命名空间和具名资源扩展能力。

### Decision: 同步观测资源标签

Prometheus 指标和健康检查中的资源名同步为 `primary_db`，避免配置名、注入名和观测标签不一致。该变更会影响现有 dashboard 和 alert 查询，因此同步更新部署观测资产。

## Risks / Trade-offs

- [Risk] 已部署环境仍使用 `AEGISCORE_RESOURCES_POSTGRES_USER_DB_*` 会导致严格配置缺少 `primary_db`。Mitigation：同步 Helm、K8s、Compose、README，并在发布说明中要求替换环境变量名。
- [Risk] 监控查询切换后，历史 `resource="user_db"` 数据不会自动重命名。Mitigation：dashboard 和 alert 使用新 label，历史查询可在 Prometheus 中按旧 label 手动回看。

## Migration Plan

1. 更新 OpenSpec change artifacts。
2. 更新 Go 代码、测试 fixture、配置样例、部署清单、文档和观测资产。
3. 运行相关 Go 测试、架构 lint 和 dashboard 检查。
4. 发布时同步替换外部 Secret/ConfigMap/Helm values 中的 PostgreSQL 环境变量名。
