# Compose 部署

本目录承载本地依赖和可选本地服务启动所需的 Docker Compose 文件。

当前提供可直接运行的本地 Compose 编排：

- `docker-compose.yml`：启动 PostgreSQL、Redis、Jaeger OTLP、本地用户服务、Prometheus 和 Grafana；RBAC seed 作为 `seed` profile 下的一次性任务按需手动执行。
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
POSTGRES_PASSWORD='<local-password>' \
AEGISCORE_AUTH_JWT_SECRET='<local-jwt-secret>' \
GRAFANA_ADMIN_PASSWORD='<local-grafana-password>' \
docker compose -f deployments/compose/docker-compose.yml up --build
```

数据库 SQL migration 已受控执行后，可按需单独执行 RBAC seed：

```bash
POSTGRES_PASSWORD='<local-password>' \
AEGISCORE_AUTH_JWT_SECRET='<local-jwt-secret>' \
docker compose -f deployments/compose/docker-compose.yml --profile seed run --rm rbac-seed
```

Compose 仅用平台 `TZ` 设置进程时区，并通过 `AEGISCORE_SERVER_HTTP_*`、`AEGISCORE_SERVER_GRPC_*`、`AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_*` 和 `AEGISCORE_RESOURCES_POSTGRES_PRIMARY_DB_*` 覆盖最终配置路径。Redis 使用统一 `TIMEOUT`，PostgreSQL 连接池使用 `POOL_*`。user-service 不声明额外队列 Redis 或支付数据库。PostgreSQL 密码必须通过调用命令或本地 `.env` 中的 `POSTGRES_PASSWORD` 注入；`POSTGRES_PASSWORD`、`AEGISCORE_AUTH_JWT_SECRET` 和 `GRAFANA_ADMIN_PASSWORD` 缺失时 Compose 会提前失败，避免用空值启动后再出现数据库鉴权或运行时错误；日志只写 stdout/stderr；本地 tracing 默认启用并固定通过 OTLP gRPC 连接 `jaeger:4317`。本地 tracing 同时保留 Ent 实体级 `ent.query` / `ent.mutation` span 和 PostgreSQL `otelsql` SQL/driver 级 span，用于本地完整链路诊断；该组合会提供更多细节，也会让 trace 更噪。

pprof 不进入 Compose 默认配置。临时诊断应在受控环境设置 `AEGISCORE_OBSERVABILITY_PPROF_ENABLED=true`、`AEGISCORE_OBSERVABILITY_PPROF_ADDR=127.0.0.1:6060`，并通过容器 namespace 内 loopback 或受控端口转发访问。

user-service 使用固定 digest 的 Distroless static nonroot 运行时镜像，容器内数值身份为 UID/GID `65532`，不包含 shell、`apk`、`wget`、`curl`、`grep` 或 Atlas。Compose healthcheck 使用 exec-form 调用镜像内原生 CLI：`/app/user-service/bin/user-service healthcheck --url http://127.0.0.1:8080/readyz`，不依赖 `CMD-SHELL` 或管道。

本地端口：

- 用户服务：http://localhost:8080
- PostgreSQL：localhost:15432 -> 容器内 5432
- Redis：localhost:16379 -> 容器内 6379
- Jaeger：http://localhost:16686，OTLP gRPC 为 localhost:4317，OTLP HTTP 为 localhost:4318
- Prometheus：http://localhost:9090
- Grafana：http://localhost:3000，账号为 `admin`，密码由 `GRAFANA_ADMIN_PASSWORD` 注入

从仓库根目录单独构建用户服务运行时镜像：

```bash
docker buildx build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-service:latest --load .
IMAGE=aegiscore-user-service:latest make user-service-image-verify
```

Compose 文件使用仓库根目录作为 build context，通过 `deployments/docker/user-service.Dockerfile` 构建用户服务运行时镜像。Dockerfile 使用 BuildKit module cache 和 Go build cache；缓存只用于加速构建，`-mod=readonly`、基础镜像 digest、Trivy 门禁和 SBOM 仍是依赖与供应链验证边界。

数据库 schema 变更流程是：Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或受控发布平台执行。Compose 不会自动执行 `atlas migrate apply`；执行 `rbac-seed` 前，应先确认本地或目标数据库已执行对应 SQL migration。普通 `aegiscore-user-service` 运行时镜像不包含 Atlas，也不支持通过 `RUN_MIGRATIONS=true` 在服务容器启动时执行 migration。
