## Why

user-service 当前将主 PostgreSQL 资源命名为 `user_db`，但该连接承载的是服务自有 OLTP 数据，包括用户、认证、角色、权限和 RBAC 数据，并不只对应用户资料表。该名称会外溢到严格配置路径、环境变量、健康检查、metrics label、部署清单和文档，容易让后续维护者误判资源边界。

将资源名统一为 `primary_db` 可以表达这是 user-service 的主读写 PostgreSQL 资源，同时保留 `resources.postgres` 的具名资源结构，便于未来按 `readonly_db`、`audit_db` 等职责扩展。

## What Changes

- 将 user-service 必需 PostgreSQL 资源名从 `user_db` 改为 `primary_db`。
- 同步 Go 常量、Fx 注入名、配置校验、RBAC CLI、测试 fixture、部署环境变量、健康检查示例、Prometheus/Grafana 观测资产和相关文档。
- 不变更数据库 schema、migration、HTTP API、认证/RBAC 行为或真实数据库名。

## Capabilities

### Modified Capabilities

- `delivery-operations`: user-service 部署和运行配置继续使用 `resources.postgres`，但 PostgreSQL 主资源名改为 `primary_db`。
- `runtime-observability`: PostgreSQL runtime dependency 指标和健康检查资源名改为 `primary_db`。

## Impact

- Go 代码：影响 user-service PostgreSQL provider、Ent provider、健康检查、metrics、RBAC CLI 和依赖注入标签。
- 部署：影响 Compose、Kubernetes、Helm 中 PostgreSQL 环境变量名。
- 观测：影响 PostgreSQL metrics 的 `resource` label、告警规则和 dashboard 查询。
- 文档：同步 README、开发、测试、架构、产品和 runbook 中的资源名。
- 数据库：不改变 Ent schema、Atlas migration 或实际 PostgreSQL 数据库内容。
