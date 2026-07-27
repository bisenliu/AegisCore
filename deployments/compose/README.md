# Compose 部署

本目录承载本地依赖和可选本地服务启动所需的 Docker Compose 文件。

当前提供可直接运行的本地 Compose 编排：

- `docker-compose.yml`：启动 PostgreSQL、Redis、Nacos、Jaeger OTLP、本地用户服务、Prometheus 和 Grafana；`deployments/docker/nacos.Dockerfile` 基于 `nacos/nacos-server:latest`，自行固定 standalone、`microservice` 功能模式和官方容器启动入口，Compose 不覆盖 Nacos 启动命令，RBAC seed 作为 `seed` profile 下的一次性任务按需手动执行。
- `deployments/docker/jaeger.Dockerfile`：保留 `jaegertracing/all-in-one:latest` 的入口与功能，只从仓库固定 digest 的 Distroless runtime 复制 `Asia/Shanghai` zoneinfo，避免 Jaeger 1.76 基础镜像缺少时区数据时忽略 `TZ`。
- `../nacos/`：Git 中唯一的本地 Nacos 配置来源，`local-host/` 与 `local-docker/` 分别保存三份完整配置，由 `tools/nacos-config-seed` 通过 Nacos v3 Admin API 发布。
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

数据库 SQL migration 已受控执行后，可按需单独执行 RBAC seed：

```bash
docker compose -f deployments/compose/docker-compose.yml --profile seed run --rm rbac-seed
```

Compose 通过 `nacos-init-host` 与 `nacos-init-docker` 两个一次性任务，分别将 `deployments/nacos/local-host/` 和 `deployments/nacos/local-docker/` 发布到 Nacos `loca-host/AEGISCORE` 与 `loca-docker/AEGISCORE`。容器内 user-service 与 RBAC seed 固定选择 `loca-docker`，主机进程选择 `loca-host`；两端均按默认顺序加载 `base.yaml`、`resources.yaml`、`user-service.yaml`。user-service 只通过 `AEGISCORE_SERVICE` 和 `AEGISCORE_NACOS_*` 选择配置来源，不挂载本地完整配置文件。日志只写 stdout/stderr；本地 tracing 默认启用并固定通过 OTLP gRPC 连接 `jaeger:4317`。本地 tracing 同时保留 Ent 实体级 `ent.query` / `ent.mutation` span 和 PostgreSQL `otelsql` SQL/driver 级 span，用于本地完整链路诊断；该组合会提供更多细节，也会让 trace 更噪。

全新 PostgreSQL volume 会显式创建 `postgres` 用户和 `aegiscore` 数据库。Compose healthcheck、`local-docker/resources.yaml` 与真实指标压测脚本都使用同一组用户和数据库名称，避免健康检查通过后服务仍因目标数据库不存在而启动失败。

需要在主机直接运行 user-service 时，只启动依赖和初始化任务：

```bash
docker compose -f deployments/compose/docker-compose.yml up -d postgres redis nacos nacos-init-host nacos-init-docker jaeger
export AEGISCORE_SERVICE=user-service
export AEGISCORE_NACOS_ADDR=127.0.0.1:8848
export AEGISCORE_NACOS_NAMESPACE=loca-host
export AEGISCORE_NACOS_GROUP=AEGISCORE
make user-service-run
```

Compose 的常驻服务和一次性任务统一显式设置 `TZ=Asia/Shanghai`。PostgreSQL 还通过启动参数固定 `timezone` 与 `log_timezone`，因此已有 data volume 重启后也会生效；该设置不删除数据，也不替代 SQL migration。Jaeger 薄镜像补齐 IANA zoneinfo，Prometheus、Redis、Grafana 和 PostgreSQL 使用各自镜像已有的 zoneinfo，Nacos 与 user-service 保持现有上海时区语义。

容器时区不改变 telemetry 的绝对时间语义：OpenTelemetry span 和 Prometheus sample 继续使用 Unix epoch。Jaeger UI 使用访问浏览器的本地时区，Prometheus Web UI 按官方设计固定显示 UTC，不能通过 Compose `TZ` 覆盖；Grafana provisioning dashboard 则固定使用 `Asia/Shanghai`，避免依赖浏览器配置。

user-service 通过 Nacos v3 Client HTTP API 读取配置，两个 `nacos-init-*` 任务通过 v3 Admin API 创建 namespace 并发布配置，因此兼容已经移除 v1/v2 API 的 Nacos 3.2。Nacos 以 `microservice` 功能模式运行，只加载 Config 和 Naming，不加载 AI 模块及其控制台入口；该模式要求 Nacos 3.2.2 或更高版本。Compose 有意使用浮动 `latest` 以持续验证最新版；正式环境应在验收后固定具体版本或镜像 digest。`nacos-data` volume 会保留本地 Nacos 数据；旧 volume 中的 `loca` Namespace 不再被当前 workload 引用，也不会由 seed 工具自动删除。

本地 Compose 显式设置 `NACOS_AUTH_ENABLE=false`、`NACOS_AUTH_ADMIN_ENABLE=false` 和 `NACOS_AUTH_CONSOLE_ENABLE=false`，分别关闭 Client、Admin API 和 Console API 鉴权，避免本地控制台操作因仅关闭部分鉴权而被拒绝。这些值只适用于隔离的本地开发网络，生产部署必须启用鉴权并使用受控凭据。

pprof 不进入 Compose 默认配置。临时诊断应修改 Nacos 中的 `observability.pprof`，并通过容器 namespace 内 loopback 或受控端口转发访问。

user-service 使用固定 digest 的 Distroless static nonroot 运行时镜像，容器内数值身份为 UID/GID `65532`，不包含 shell、`apk`、`wget`、`curl`、`grep` 或 Atlas。Compose healthcheck 使用 exec-form 调用镜像内原生 CLI：`/app/user-service/bin/user-service healthcheck --url http://127.0.0.1:8080/readyz`，不依赖 `CMD-SHELL` 或管道。

本地端口：

- 用户服务：http://localhost:8080
- PostgreSQL：localhost:15432 -> 容器内 5432
- Redis：localhost:16379 -> 容器内 6379
- Nacos Console：http://localhost:8849，API 为 localhost:8848，Nacos gRPC 为 localhost:9848
- Jaeger：http://localhost:16686，OTLP gRPC 为 localhost:4317，OTLP HTTP 为 localhost:4318
- Prometheus：http://localhost:9090
- Grafana：http://localhost:3000，账号为 `admin`，本地默认密码来自 `grafana/grafana.ini`

从仓库根目录单独构建用户服务运行时镜像：

```bash
docker buildx build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-service:latest --load .
make user-service-image-verify
```

Compose 文件使用仓库根目录作为 build context，通过 `deployments/docker/user-service.Dockerfile` 构建用户服务运行时镜像。Dockerfile 使用 BuildKit module cache 和 Go build cache；缓存只用于加速构建，`-mod=readonly`、基础镜像 digest、Trivy 门禁和 SBOM 仍是依赖与供应链验证边界。

数据库 schema 变更流程是：Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或受控发布平台执行。Compose 不会自动执行 `atlas migrate apply`；执行 `rbac-seed` 前，应先确认本地或目标数据库已执行对应 SQL migration。普通 `aegiscore-user-service` 运行时镜像不包含 Atlas，也不支持通过 `RUN_MIGRATIONS=true` 在服务容器启动时执行 migration。
