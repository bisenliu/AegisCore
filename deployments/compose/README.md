# Compose 部署

本目录承载本地依赖和可选本地服务启动所需的 Docker Compose 文件。

当前提供可直接运行的本地 Compose 编排：

- `docker-compose.yml`：启动 PostgreSQL、Redis、用户服务、Prometheus 和 Grafana，并在数据库 SQL migration 已受控执行后先执行 RBAC seed，再启动用户服务。
- `prometheus/prometheus.yml`：抓取用户服务 `/metrics`，并加载 `deployments/observability/prometheus/user-service-alerts.yaml`。
- `grafana/provisioning/`：自动配置 Prometheus datasource 和用户服务看板。
- `grafana/dashboards/user-service-overview.json`：由 `deployments/observability/grafana/user-service-overview.json` 生成的本地自动导入副本，datasource uid 固定为 `prometheus`；不要手动编辑该文件。

更新 Grafana 看板时，只修改通用源文件 `deployments/observability/grafana/user-service-overview.json`，然后从仓库根目录生成 Compose 版本：

```bash
make compose-dashboard-generate
```

提交前可检查生成文件是否已同步：

```bash
make compose-dashboard-check
```

从仓库根目录启动：

```bash
docker compose -f deployments/compose/docker-compose.yml up --build
```

本地端口：

- 用户服务：http://localhost:8080
- PostgreSQL：localhost:15432 -> 容器内 5432
- Redis：localhost:16379 -> 容器内 6379
- Prometheus：http://localhost:9090
- Grafana：http://localhost:3000，默认账号密码为 `admin` / `admin`

从仓库根目录单独构建用户服务运行时镜像：

```bash
docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .
```

Compose 文件使用仓库根目录作为 build context，通过 `deployments/docker/user-service.Dockerfile` 构建用户服务运行时镜像。

数据库 schema 变更流程是：Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或受控发布平台执行。Compose 不会自动执行 `atlas migrate apply`；启动 `rbac-seed` 和 `user-service` 前，应先确认本地或目标数据库已执行对应 SQL migration。普通 `aegiscore-user-services` 运行时镜像不包含 Atlas，也不支持通过 `RUN_MIGRATIONS=true` 在服务容器启动时执行 migration。
