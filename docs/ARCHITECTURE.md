# AegisCore 架构说明

## 1. 总体结构

AegisCore 是 Go 1.26 workspace，当前由四个主要部分组成：

| 模块 | 职责 |
|---|---|
| `common/` | 跨服务稳定契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力 |
| `user-service/` | 用户服务 CLI、Fx/Gin runtime、feature 业务代码、Ent schema、migration 和 OpenAPI 生成 |
| `tools/` | 仓库级交付工具，例如跨服务复用的 OpenAPI 转换 CLI |
| `deployments/` | Docker、Compose、Kubernetes、Helm、Prometheus 和 Grafana 部署观测资产 |

根 `Makefile` 汇总构建、测试、lint、verify、migration、OpenAPI 和 dashboard 命令。README 将 `docs/` 与 `openspec/` 定义为协作和规格入口。

## 2. `common/` 共享基础

`common/` 是跨服务复用层，不依赖 `user-service` 的 feature 实现。

| 路径 | 职责 |
|---|---|
| `common/contract/errors/` | 统一错误码、错误对象和转换 |
| `common/contract/response/` | 统一响应 envelope 和消息 |
| `common/contract/pagination/` | 分页数据结构和 helper |
| `common/http/binding/` | Gin 请求绑定与校验衔接 |
| `common/http/middleware/` | auth、casbin、cors、logging、metrics、recovery 和 span error |
| `common/http/openapi/` | OpenAPI 转换和渲染 helper |
| `common/http/response/` | HTTP 响应写入和错误响应 |
| `common/runtime/config/` | 仅包含 app/server/log/observability 的跨服务核心配置、严格 YAML loader 和 validation primitive |
| `common/runtime/resources/` | 无业务语义的 Redis/PostgreSQL 资源类型、默认值和校验；具名资源由服务声明 |
| `common/runtime/datastore/` | Postgres、Redis 和 Fx provider |
| `common/runtime/logger/` | 写 stdout/stderr 的 zap logger |
| `common/runtime/observability/` | metrics 与 tracing provider |
| `common/runtime/scheduler/` | scheduler、lock、metrics 和 logger |
| `common/runtime/workerpool/` | 固定 worker pool、stats 和 errors |
| `common/security/` | JWT verifier、token version validator 契约、Casbin authorizer、password hash |
| `common/testing/` | Postgres/Redis Testcontainers 和 fixtures |
| `common/validation/` | validator、翻译、字段和错误 |

## 3. `user-service` 运行入口

`user-service/cmd/main.go` 定义 `aegiscore-user-services` CLI：

- `serve --config <path>`：启动 Fx app 和 Gin HTTP server。
- `rbac seed`：初始化默认系统角色、权限和绑定。
- `rbac assign-super-admin --user-id <uuid>`：为已有用户绑定超级管理员角色。
- `rbac create-super-admin`：创建或复用管理员用户并绑定超级管理员角色。
- `fxgraph`：生成 Fx 依赖图。
- `healthcheck --url <url> --timeout <duration>`：在容器内无 shell、wget、curl 或 grep 依赖地检查 `/readyz`。

`user-service/internal/bootstrap/` 构造应用、HTTP server 和默认关闭的独立 pprof 诊断监听，并通过 `AppOptions` 接收 CLI 已解析的 service config、派生共享 runtime config 和组装 Fx options。`user-service/internal/config/` 拥有服务根配置、认证/RBAC feature cache、Ent 配置、具名 resources 和服务级校验，并复用 `common/runtime/config` 的严格 loader。`user-service/internal/providers/` 提供 Gin、Ent、Postgres、Redis、auth verifier、metrics、health 和 routes provider，不读取配置文件。

## 4. HTTP 路由结构

`user-service/internal/router/router.go` 是 HTTP 路由聚合点：

1. 注册健康检查。
2. 注册 OpenAPI。
3. 按配置注册 metrics。
4. 挂载 `/api/v1` 业务路由。

pprof 不挂载到业务 router。临时诊断时通过 `PPROF_ENABLED=true` 和 `PPROF_ADDR=127.0.0.1:6060` 启动独立监听，并只通过 loopback、`kubectl port-forward` 或等价受控通道访问。Gin 默认不信任代理；真实客户端地址和 forwarded headers 由 Ingress、gateway 或 service mesh 的入口安全策略负责。

业务路由分层：

- `/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/auth/change-password` 由认证公开路由挂载。
- 其余受保护路由先通过 `AuthWithTokenVersionValidator`。
- 权限、角色和用户接口再通过 `permissionhttp.Authorize` 执行 RBAC 授权。
- 用户 API 位于 `/api/v1/users`。
- 权限 API 位于 `/api/v1/permissions`。
- 角色 API 位于 `/api/v1/roles` 和 `/api/v1/users/:user_id/roles`。

## 5. Feature 分层

`user-service/internal/features/` 以能力划分 feature：

| Feature | 主要职责 |
|---|---|
| `auth` | 登录、刷新、退出、改密、会话、token、凭证和 token version |
| `permission` | 权限目录、路由差异、有效权限、Casbin policy 和授权中间件 |
| `role` | 角色、角色权限、用户角色、系统 seed 和超级管理员绑定 |
| `user` | 用户资料创建、查询、列表、状态和存储 |

典型 feature 内部结构：

- `domain/`：领域对象、值对象和领域错误。
- `application/`：命令、查询、服务、端口、validator 和 metrics。
- `infrastructure/`：Postgres、Redis、Casbin 等适配器。
- `transport/http/`：controller、request、response、mapper、routes 和输入校验。

`domain/` 和 `application/` 生产代码保持框架无关，不承载仅服务于 Fx DI 的 import、`fx.In` 或 `name`/`optional` tag；无 DI metadata 需求的普通 application 构造器由 feature 根 `fx.go` 直接注册，确有 named/optional 等 metadata 或配置转换需求时才由 composition adapter 转换。分层约束由 `user-service/scripts/architecture-lint.sh` 检查。

## 6. 核心流程

### 6.1 服务启动

1. `aegiscore-user-services serve --config ./configs/config.yaml` 进入 `runServe`，CLI 单次解析并校验 service config。
2. `bootstrap.NewApp(cfg)` 通过 `AppOptions` supply 同一个 service config 及其派生的共享 runtime config，并组装 logger、datastore、auth、metrics、health、routes 和 HTTP server。
3. `fx.New` 同步构建依赖图、执行 invoke 及其 constructor 依赖；该阶段不受 `runtime.lifecycle.start_timeout` 限制。
4. CLI 使用同一配置值建立显式 Start context 并调用 `App.Start`，该 context 约束全部 `OnStart` hooks。
5. 收到外部终止信号或内部 Fx shutdown signal 后，CLI 使用同一配置值建立显式 Stop context 并调用一次 `App.Stop`。

### 6.2 登录和会话

1. HTTP 请求进入 `auth/transport/http` controller。
2. application command 校验输入、凭证和用户状态。
3. session store 写入 Redis 会话状态。
4. user-service 私有 token issuer 签发 access token 和 refresh token。
5. 响应通过共享 response helper 返回统一 envelope。

### 6.3 受保护 API 授权

1. 请求进入 `/api/v1` authenticated group。
2. 共享 HTTP auth middleware 调用最小 access token verifier 接口，user-service verifier 校验 JWT、access subject、认证 claims 和 token version。
3. RBAC 中间件读取当前用户和请求资源。
4. permission authorizer 使用 Casbin 或同步后的 policy 判断访问权限。
5. 通过后进入目标 controller。

### 6.4 RBAC seed 和超级管理员

1. `rbac seed` 加载 user-service 私有配置，按服务私有资源名打开 user DB，创建 Ent client。
2. role seed service 创建或更新系统角色、权限和绑定。
3. `create-super-admin` 读取 `ADMIN_PASSWORD`，创建或复用用户，按需更新密码。
4. seed service 绑定内置超级管理员角色。

### 6.5 数据迁移

1. Ent schema 变化后执行 `make user-service-generate`。
2. 使用 `make user-service-migrate-diff name=<migration-name>` 生成 Atlas migration。
3. 使用 `make user-service-migrate-validate` 校验 migration。
4. 使用 `atlas migrate hash` 或等价流程刷新 `atlas.sum`，将 SQL migration 与权限要求提交 Git。
5. 发布时通过 DBA 工单或受控发布平台人工或受控执行 SQL migration；`CREATE EXTENSION IF NOT EXISTS pg_trgm;` 等扩展语句可能需要 DBA 权限或前置动作。

## 7. 部署和观测

共享核心配置只含 `app/server/log/observability`。Redis/PostgreSQL 类型由 `common/runtime/resources` 提供，user-service 在 `resources.redis.cache_redis` 与 `resources.postgres.user_db` 声明实际资源；feature cache 由 `auth.token_version_cache` 和 `rbac.user_role_cache` 各自拥有。日志输出到 stdout/stderr，tracing 启用后固定使用 OTLP，进程时区由平台 `TZ` 控制。

- Dockerfile：`deployments/docker/user-service.Dockerfile` 使用 BuildKit manifest-first 依赖层、只读 Go module 解析、静态编译和固定 digest 的 `gcr.io/distroless/static-debian12:nonroot` 运行时；运行镜像身份为 UID/GID `65532`，不包含 shell、包管理器、下载工具或 Atlas。
- Compose：`deployments/compose/docker-compose.yml` 继承 Distroless `nonroot` 身份，user-service healthcheck 使用 exec-form 调用原生 `healthcheck` CLI。
- Kubernetes：`deployments/k8s/user-services/` 使用 UID/GID/fsGroup `65532`、只读根文件系统、`/tmp` emptyDir 和 kubelet HTTP probes。
- Helm：`deployments/helm/aegiscore-user-services/` 渲染与原生 YAML 一致的 UID/GID `65532`、HTTP probes 和 RBAC seed Job。
- CI：阻塞式 `test` job 设置 `AEGISCORE_TEST_CONTAINERS=1` 运行真实 PostgreSQL/Redis 测试；镜像安全 job 复用同一 BuildKit image ID 执行内容断言、Trivy HIGH/CRITICAL 门禁和 SBOM 生成。
- Prometheus alerts：`deployments/observability/prometheus/user-service-alerts.yaml`。
- Grafana dashboard：`deployments/observability/grafana/user-service-overview.json`。
- Compose dashboard 由 `deployments/compose/scripts/generate-grafana-dashboard.sh` 从通用观测 dashboard 生成。

## 8. 规格边界

稳定能力位于 `openspec/specs/`。跨 feature、跨模块、外部契约、schema、部署或行为变更必须先创建 `openspec/changes/<change-name>/`，完成后归档回主规格。
