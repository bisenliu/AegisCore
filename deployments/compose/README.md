# Compose 部署

本目录承载本地依赖和可选本地服务启动所需的 Docker Compose 文件。

当前提供可直接运行的本地 Compose 编排：

- `docker-compose.yml`：启动 PostgreSQL、Redis、用户服务、Prometheus 和 Grafana，并在用户服务启动前执行 Atlas migration 与 RBAC seed。
- `prometheus/prometheus.yml`：抓取用户服务 `/metrics`，并加载 `deployments/observability/prometheus/user-service-alerts.yaml`。
- `grafana/provisioning/`：自动配置 Prometheus datasource 和用户服务看板。
- `grafana/dashboards/user-service-overview.json`：基于 `deployments/observability/grafana/user-service-overview.json` 的本地自动导入副本，datasource uid 固定为 `prometheus`。

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

从仓库根目录单独构建用户服务镜像：

```bash
docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .
```

Compose 文件使用仓库根目录作为 build context，并通过 `deployments/docker/user-service.Dockerfile` 构建用户服务镜像。
