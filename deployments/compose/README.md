# Compose 部署

本目录承载本地依赖和可选本地服务启动所需的 Docker Compose 文件。

当前提供可直接运行的本地 Compose 编排：

- `docker-compose.yml`：启动 PostgreSQL、Redis、用户服务、Prometheus 和 Grafana，并通过独立 `user-service-migrate` one-shot 服务在用户服务启动前执行 Atlas migration，再执行 RBAC seed。
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

从仓库根目录单独构建 migration 镜像：

```bash
docker build -f deployments/docker/user-service-migration.Dockerfile -t aegiscore-user-services-migration .
```

Compose 文件使用仓库根目录作为 build context，并分别通过 `deployments/docker/user-service.Dockerfile` 构建用户服务运行时镜像、通过 `deployments/docker/user-service-migration.Dockerfile` 构建 Atlas/migration 镜像。

Compose 中的 `user-service-migrate` 是本地 release migration job 的模拟：它使用专用 `aegiscore-user-services-migration` 镜像执行 `atlas migrate apply`，成功后才允许 `rbac-seed` 和 `user-service` 启动。普通 `aegiscore-user-services` 运行时镜像不包含 Atlas，也不支持通过 `RUN_MIGRATIONS=true` 在服务容器启动时执行 migration。生产多副本发布也应使用独立 migration Job 或 CI/CD release job，而不是依赖服务副本启动时迁移。
