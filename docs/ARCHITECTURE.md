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
| `common/http/client/` | 基于可复用 Resty client 的业务中立出站请求、timeout、代理与状态错误 helper |
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

Scheduler 对外只暴露固定 key 的注册/删除和生命周期操作，以 nil/non-nil lock、renew policy 以及 `WaitTimeout` 表达策略。内部不可导出的 pipeline 固定串联本地 overlap、全局并发、Redis lock、任务 context、续租与结果观测，每个 stage 通过局部 `defer` 释放自身资源；completed/failed duration 从任务 started 计算，不包含 gate 或 lock wait。全局/锁 wait 在高频且允许 overlap 时可能积累等待 goroutine，不等价于有界任务队列。Redis owner-token lock 仍是 lease，不提供 exactly-once 或 fencing 保证。

## 3. `user-service` 运行入口

`user-service/cmd/main.go` 定义 `aegiscore-user-service` CLI：

- `serve`：通过 `AEGISCORE_SERVICE` 和 `AEGISCORE_NACOS_*` 定位 Nacos 分层配置，启动 Fx app 和 Gin HTTP server。
- `rbac seed`：初始化默认系统角色、权限和绑定。
- `rbac bootstrap-super-admin`：为全新数据库一次性创建系统内置固定 ID 的初始超级管理员用户并绑定超级管理员角色。
- `fxgraph`：生成 Fx 依赖图。
- `config validate|render|sources`：验证 Nacos 配置、脱敏渲染最终配置和展示实际来源。
- `healthcheck --url <url> --timeout <duration>`：在容器内无 shell、wget、curl 或 grep 依赖地检查 `/readyz`。

`user-service/internal/bootstrap/` 构造应用、HTTP server 和默认关闭的独立 pprof 诊断监听，并通过 `AppOptions` 接收 CLI 已解析的 service config、派生共享 runtime config 和组装 Fx options。`user-service/internal/config/` 拥有服务根配置、认证/RBAC feature cache、Ent 配置、具名 resources 和服务级校验，并复用 `common/runtime/config` 的 Nacos 来源解析、YAML deep merge、strict decode、digest 和脱敏能力。`user-service/internal/providers/` 是服务级 provider 汇总入口，只组合子模块，不承载具体 provider 构造器；`providers/datastore/` 承载 PostgreSQL、Redis、Ent client、Ent plugins、Ent SQL log、Ent metrics 和 Ent tracing 接线；`providers/observability/` 承载 health checks、runtime dependency metrics、metrics provider 和 tracing provider 接线；`providers/security/` 承载 JWT service、认证 token policy 和 password service 接线；`providers/transport/` 承载 Gin mode、Gin engine、routes 和 API rate limiters 接线。providers 不读取配置来源。版本化本地 Nacos 配置位于 `deployments/nacos/local-host/` 与 `deployments/nacos/local-docker/`，每个目录都是对应 Namespace 的完整三文档发布源；`tools/nacos-config-seed` 只负责将指定目录发布到指定 Namespace。

## 4. HTTP 路由结构

`user-service/internal/router/router.go` 是 HTTP 路由聚合点：

1. 注册健康检查。
2. 注册 OpenAPI。
3. 按配置注册 metrics。
4. 挂载 `/api/v1` 业务路由。

pprof 不挂载到业务 router。临时诊断时修改 Nacos 中的 `observability.pprof` 配置，并只通过 loopback、`kubectl port-forward` 或等价受控通道访问。Gin 默认不信任代理；服务只通过 `server.http.trusted_proxies` 信任显式配置的上游代理 IP/CIDR，并使用 Gin trusted proxy 机制解析真实客户端地址。Ingress、gateway 或 service mesh 必须在入口边界覆盖或重建 forwarded headers，不能透传客户端提供的未清洗值。

业务路由分层：

- `/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/auth/force-change-password` 由认证公开路由挂载。
- 其余受保护路由先通过 `AuthWithTokenVersionValidator`。
- 权限、角色和用户接口再通过 `permissionhttp.Authorize` 执行 RBAC 授权。
- 用户 API 位于 `/api/v1/users`。
- 权限 API 只提供 `/api/v1/permissions` 列表和 `/api/v1/permissions/users/:user_id/effective` 有效权限查询。
- 角色 API 位于 `/api/v1/roles` 和 `/api/v1/users/:user_id/roles`。

## 5. Feature 分层

`user-service/internal/features/` 以能力划分 feature：

| Feature | 主要职责 |
|---|---|
| `auth` | 登录、刷新、退出、改密、会话、token、凭证和 token version |
| `permission` | 代码权限目录的只读投影、有效权限、Casbin policy 和授权中间件 |
| `role` | 角色、角色权限、用户角色、系统 seed 和初始超级管理员 bootstrap |
| `user` | 用户资料创建、查询、列表、状态和存储 |

典型 feature 内部结构：

- `domain/`：领域对象、值对象和领域错误。
- `application/`：命令、查询、服务、端口、validator 和 metrics。
- `infrastructure/`：Postgres、Redis、Casbin 等适配器。
- `transport/http/`：controller、request、response、mapper、routes 和输入校验。

`domain/` 和 `application/` 生产代码保持框架无关，不承载仅服务于 Fx DI 的 import、`fx.In` 或 `name`/`optional` tag；无 DI metadata 需求的普通 application 构造器由 feature 根 `fx.go` 直接注册，确有 named/optional 等 metadata 或配置转换需求时才由 composition adapter 转换。分层约束由 `user-service/scripts/architecture-lint.sh` 检查。

真实外部 HTTP 系统的消费侧端口、DTO、认证、retry policy、业务错误映射和防腐逻辑仍位于 `user-service/internal/integration/http` 或所属 feature；`common/http/client` 只提供 Resty client 复用、单次请求构造与发送原语，消费侧可注入预配置的 Resty client。

## 6. 核心流程

### 6.1 服务启动

1. `aegiscore-user-service serve` 进入 `runServe`，CLI 从环境变量读取 Nacos 来源，按 dataId 加载 YAML、deep merge、strict decode、ApplyDefaults 并校验 service config。
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

在线 RBAC 写入提交后，Redis Pub/Sub 只向各副本发送快速唤醒 hint，PostgreSQL latest policy revision 是恢复与收敛的唯一权威事实。permission Redis watcher 在单一根生命周期中并行监督可重建的 Pub/Sub subscription 和启动立即执行、随后周期执行的数据库 revision 校准；订阅确认或 Receive 失败只进入有界退避重连，不停止权威校准。消息处理与周期校准在根循环中串行执行，防止 Casbin projection 并发倒退。readiness 依据最后一次完整权威校准的 staleness 判定，当前订阅正在重连但校准仍新鲜时不制造 watcher 粘滞失败。

### 6.4 RBAC seed 和超级管理员 bootstrap

1. `rbac seed` 加载 user-service 私有配置，按服务私有资源名打开 user DB，创建 Ent client。
2. role seed service 按 `rbacbaseline.DefaultPermissions()` 的稳定 `permission_id` 创建或更新权限投影，并维护系统角色和默认绑定。
3. 权限定义变更随代码发布；路由测试构建真实 Gin route graph，与代码基线双向比较，missing 或 stale 均阻断 CI。
4. `bootstrap-super-admin` 读取 `ADMIN_BOOTSTRAP_PASSWORD`，使用 `rbacbaseline.BootstrapSuperAdminUserID` 创建 `MustChangePassword` 用户并绑定内置超级管理员角色。
5. 后续超级管理员授权通过在线用户角色绑定 API 完成，由在线流程负责 policy version 发布和缓存收敛。

系统内置 RBAC 角色、权限和 bootstrap 用户 ID 由 `user-service/internal/shared/rbacbaseline/ids.go` 统一定义为手写固化 UUID 字符串，semantic name 绑定稳定业务授权语义，不绑定项目展示名、HTTP path、中文文案或 Go symbol。普通运行时业务实体仍使用 `common/runtime/id.NewUUID()` 生成 UUID v7；已有项目重命名不得默认修改、重算或复用系统内置 ID。

### 6.5 数据迁移

1. Ent schema 变化后执行 `make user-service-generate`。
2. 使用 `make user-service-migrate-diff name=<migration-name>` 生成 Atlas migration。
3. 使用 `make user-service-migrate-validate` 校验 migration。
4. 使用 `atlas migrate hash` 或等价流程刷新 `atlas.sum`，将 SQL migration 与权限要求提交 Git。
5. `users.nickname` substring 模糊查询统一使用 `pg_trgm` 提供的 GIN `gin_trgm_ops` 索引，不保留普通索引、无扩展 fallback 或双索引兼容分支；发布时通过 DBA 工单或受控发布平台人工或受控执行 SQL migration，并确认 `CREATE EXTENSION IF NOT EXISTS pg_trgm;` 所需的 DBA 权限或前置动作。
6. 删除权限时 migration 先删除 `role_permissions` 再删除 `permissions`；随后执行同版本 RBAC seed，并通过显式 reload 或滚动重启收敛 Casbin policy。

## 7. 部署和观测

共享核心配置只含 `app/runtime/server/log/observability`。Redis/PostgreSQL 类型由 `common/runtime/resources` 提供，user-service 在 `resources.redis.cache_redis` 与 `resources.postgres.primary_db` 声明实际资源；Redis 通过 `mode` 选择 `cluster` 或 `standalone`，集群使用 `addrs`，非集群使用 `addr`，两种 mode 均固定使用 Redis 0 号库且不暴露 `db` 配置。feature cache 由 `auth.token_version_cache` 和 `rbac.user_role_cache` 各自拥有。日志输出到 stdout/stderr，tracing 启用后固定使用 OTLP，进程时区由已校验的 `runtime.timezone` 配置控制，不读取平台 `TZ`。

- Dockerfile：`deployments/docker/user-service.Dockerfile` 使用 BuildKit manifest-first 依赖层、只读 Go module 解析、静态编译和固定 digest 的 `gcr.io/distroless/static-debian12:nonroot` 运行时；运行镜像身份为 UID/GID `65532`，不包含 shell、包管理器、下载工具或 Atlas。
- Compose：`deployments/compose/docker-compose.yml` 继承 Distroless `nonroot` 身份，user-service healthcheck 使用 exec-form 调用原生 `healthcheck` CLI。
- 本地 Nacos：`deployments/nacos/` 是 Git 权威来源；两个 Compose 初始化任务分别将 `local-host/` 与 `local-docker/` 的完整三文档发布到 `loca-host` 与 `loca-docker`。
- Kubernetes：`deployments/k8s/user-service/` 使用 UID/GID/fsGroup `65532`、只读根文件系统、`/tmp` emptyDir 和 kubelet HTTP probes。
- Helm：`deployments/helm/aegiscore-user-service/` 渲染与原生 YAML 一致的 UID/GID `65532`、HTTP probes 和 RBAC seed Job，并要求生产发布显式传入不可变 `image.ref`。
- CI：主 `ci.yml` 唯一触发仅支持 `workflow_call` 的质量 workflow，使同一 commit 的 lint 与普通单测各运行一次；独立 `container-test` job 通过根 `make test-containers` 显式运行 common 与 user-service 的真实 PostgreSQL/Redis 测试；镜像安全 job 复用同一 BuildKit image ID 执行内容断言、Trivy HIGH/CRITICAL 门禁和 SBOM 生成。
- Prometheus alerts：`deployments/observability/prometheus/user-service-alerts.yaml`。
- Grafana dashboard：`deployments/observability/grafana/user-service-overview.json`。
- Compose dashboard 由 `deployments/compose/scripts/generate-grafana-dashboard.sh` 从通用观测 dashboard 生成。

## 8. 规格边界

稳定能力位于 `openspec/specs/`。跨 feature、跨模块、外部契约、schema、部署或行为变更必须先创建 `openspec/changes/<change-name>/`，完成后归档回主规格。
