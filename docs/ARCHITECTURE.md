# AegisCore 架构说明

## 1. 总体结构

AegisCore 是 Go 1.27.4 workspace，当前由四个主要部分组成：

| 模块 | 职责 |
|---|---|
| `platform/` | 跨服务稳定契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力 |
| `user-service/` | 当前用户服务，位于仓库根目录；后续业务服务同样与它平级，不集中到 `services/` |
| `tools/` | 仓库级交付工具，例如跨服务复用的 OpenAPI 转换 CLI |
| `deployments/` | Docker、Compose、Helm、Prometheus 和 Grafana 部署观测资产；Kubernetes 清单只由 Helm 渲染 |

根 `Makefile` 通过 `BUSINESS_SERVICES` 汇总已登记根目录业务服务的构建、测试、lint、verify、migration 和 OpenAPI 命令。新增服务时把目录名追加到该清单，并在 `go.work` 登记对应 module；服务私有命令仍保留 `<service>-` 前缀。业务服务之间不得互相 import。README 将 `docs/` 与 `openspec/` 定义为协作和规格入口。

## 2. `platform/` 共享基础

`platform/` 是跨服务复用层，不依赖 `user-service` 的 feature 实现。

| 路径 | 职责 |
|---|---|
| `platform/contract/errors/` | 统一错误码、错误对象和转换 |
| `platform/contract/response/` | 统一响应 envelope 和消息 |
| `platform/contract/pagination/` | 分页数据结构和 helper |
| `platform/http/binding/` | Gin 请求绑定与校验衔接 |
| `platform/http/client/` | 基于可复用 Resty client 的业务中立出站请求、timeout、代理与状态错误 helper |
| `platform/http/middleware/` | auth、casbin、cors、logging、metrics、recovery 和 span error |
| `platform/http/openapi/` | OpenAPI 转换和渲染 helper |
| `platform/http/response/` | HTTP 响应写入和错误响应 |
| `platform/runtime/config/` | 仅包含 app/server/log/observability 的跨服务核心配置、严格 YAML loader 和 validation primitive |
| `platform/runtime/resources/` | 无业务语义的 Redis/PostgreSQL 资源类型、默认值和校验；具名资源由服务声明 |
| `platform/runtime/datastore/` | Postgres、Redis 和 Fx provider |
| `platform/runtime/httpserver/` | 业务中立的 `net/http` 同步监听、异步服务、异常分类、优雅关闭、强制关闭和 handler drain 生命周期 |
| `platform/runtime/redispubsub/` | Redis classic Pub/Sub 单 channel 订阅确认、阻塞接收、有界背压、退避重连和单向生命周期 |
| `platform/runtime/logger/` | 写 stdout/stderr 的 zap logger |
| `platform/runtime/observability/` | metrics 与 tracing provider |
| `platform/runtime/scheduler/` | scheduler、lock、metrics 和 logger |
| `platform/runtime/workerpool/` | 固定 worker pool、stats 和 errors |
| `platform/security/` | JWT verifier、token version validator 契约、Casbin authorizer、password hash |
| `platform/testing/` | Postgres/Redis Testcontainers 和 fixtures |
| `platform/validation/` | validator、翻译、字段和错误 |

Scheduler 对外只暴露固定 key 的注册/删除和生命周期操作，以 nil/non-nil lock、renew policy 以及 `WaitTimeout` 表达策略。内部不可导出的 pipeline 固定串联本地 overlap、全局并发、Redis lock、任务 context、续租与结果观测，每个 stage 通过局部 `defer` 释放自身资源；completed/failed duration 从任务 started 计算，不包含 gate 或 lock wait。全局/锁 wait 在高频且允许 overlap 时可能积累等待 goroutine，不等价于有界任务队列。Redis owner-token lock 仍是 lease，不提供 exactly-once 或 fencing 保证。

`platform/runtime/redispubsub` 只管理一个 Redis classic Pub/Sub channel 的 subscription lifecycle：调用方必须显式提供全部 options，subscriber 等待订阅确认后阻塞接收，以固定容量 channel 施加 context-aware 背压，并在失败后按带抖动的有界指数退避重连。每个 attempt 只关闭自己拥有的 PubSub，不关闭共享 Redis client。该 primitive 不提供 publish envelope、revision、outbox、数据库校准、缓存失效、pattern/sharded subscription、Redis Streams 或可靠投递；Redis Pub/Sub 仍是可丢失的 at-most-once 通知。

`platform/runtime/httpserver` 只管理一个 `net/http` server 的生命周期：`New` 严格校验调用方提供的全部 options 且不产生监听或 goroutine，`Start` 同步绑定地址后异步 `Serve`，`Stop` 以唯一后台 cleanup 统一执行 Shutdown、listener 显式关闭、必要时强制 Close、handler drain 和 Serve goroutine 等待。实例从 created 进入 running，异常 Serve 可进入 failed，停止后固定进入 stopped 且不能重启；并发 Stop 共享最终结果，单个调用方 context 到期不会取消后台 cleanup。该 primitive 不依赖 Gin、Fx、服务配置或业务日志，也不管理 hijacked connection、WebSocket、进程外负载均衡 drain 或无法协作退出的 goroutine。

## 3. `user-service` 运行入口

`user-service/cmd/main.go` 定义 `aegiscore-user-service` CLI：

- `serve`：通过 `AEGISCORE_SERVICE` 和 `AEGISCORE_NACOS_*` 定位 Nacos 分层配置，启动 Fx app 和 Gin HTTP server。
- `rbac seed`：初始化默认系统角色、权限和绑定。
- `rbac bootstrap-super-admin`：为全新数据库一次性创建系统内置固定 ID 的初始超级管理员用户并绑定超级管理员角色。
- `fxgraph`：生成 Fx 依赖图。
- `config validate|render|sources`：验证 Nacos 配置、脱敏渲染最终配置和展示实际来源。
- `healthcheck --url <url> --timeout <duration>`：在容器内无 shell、wget、curl 或 grep 依赖地检查 `/readyz`。

`user-service/internal/bootstrap/` 构造应用，并以 `HTTPRuntime` 和 `PprofRuntime` 显式表达业务 HTTP 与默认关闭的独立 pprof 诊断监听。bootstrap 只负责 enabled 策略、配置到 `httpserver.Options` 的映射、Fx hook、服务日志和异常退出信号，不拥有通用 listener、Serve、Shutdown、Close 或 drain 状态机；两个 runtime 各自持有独立的 `httpserver.Managed`。`AppOptions` 接收 CLI 已解析的 service config、派生共享 runtime config 和组装 Fx options。`user-service/internal/config/` 拥有服务根配置、认证/RBAC feature cache、Ent 配置、具名 resources 和服务级校验，并复用 `platform/runtime/config` 的 Nacos 来源解析、YAML deep merge、strict decode、digest 和脱敏能力。`user-service/internal/providers/` 是服务级 provider 汇总入口，只组合子模块，不承载具体 provider 构造器；`providers/datastore/` 承载 PostgreSQL、Redis、Ent client、Ent plugins、Ent SQL log、Ent metrics 和 Ent tracing 接线；PostgreSQL、Redis 与 Ent constructor 接收 composition root 传入的资源名，不写死名称，也不遍历配置自动连接未登记资源；`providers/observability/` 承载 health checks、runtime dependency metrics、metrics provider 和 tracing provider 接线；`providers/security/` 承载 JWT service、认证 token policy 和 password service 接线；`providers/transport/` 承载 Gin mode、Gin engine、routes 和 API rate limiters 接线。providers 不读取配置来源。版本化本地 Nacos 配置位于 `deployments/nacos/local-host/` 与 `deployments/nacos/local-docker/`，每个目录都是对应 Namespace 的完整三文档发布源；`tools/nacos-config-seed` 只负责将指定目录发布到指定 Namespace。

## 4. HTTP 路由结构

`user-service/internal/router/router.go` 是 HTTP 路由聚合点：

1. 注册健康检查。
2. 注册 OpenAPI。
3. 按配置注册 metrics。
4. 挂载 `/api/v1` 业务路由。

pprof 不挂载到业务 router。临时诊断时修改 Nacos 中的 `observability.pprof` 配置，并只通过 loopback、`kubectl port-forward` 或等价受控通道访问。Gin 默认不信任代理；服务只通过 `server.http.trusted_proxies` 信任显式配置的上游代理 IP/CIDR，并使用 Gin trusted proxy 机制解析真实客户端地址。Ingress、gateway 或 service mesh 必须在入口边界覆盖或重建 forwarded headers，不能透传客户端提供的未清洗值。跨域访问由 `http.cors` 配置结构显式控制，支持按源、方法、头部和凭证白名单挂载 CORS 中间件；未启用时（生产推荐由 API 网关统管）不挂载中间件，避免不透明隐式放行。

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

`domain/` 和 `application/` 生产代码保持框架无关，不承载仅服务于 Fx DI 的 import、`fx.In` 或 `name`/`optional` tag；无 DI metadata 需求的普通 application 构造器由 feature 根 `fx.go` 直接注册，确有 named/optional 等 metadata 或配置转换需求时才由 composition adapter 转换。Go import 分层约束由根 `.golangci.yml` 的 `depguard` 检查，DI metadata 和生成物 drift 等跨文件约束由 `user-service/scripts/architecture/lint.sh` 检查。

真实外部 HTTP 系统的消费侧端口、DTO、认证、retry policy、业务错误映射和防腐逻辑仍位于 `user-service/internal/integration/http` 或所属 feature；`platform/http/client` 只提供 Resty client 复用、单次请求构造与发送原语，消费侧可注入预配置的 Resty client。

## 6. 核心流程

### 6.1 服务启动

1. `aegiscore-user-service serve` 进入 `runServe`，CLI 从环境变量读取 Nacos 来源，按 dataId 加载 YAML、deep merge、strict decode、ApplyDefaults 并校验 service config。
2. `bootstrap.NewApp(cfg)` 通过 `AppOptions` supply 同一个 service config 及其派生的共享 runtime config，并组装 logger、datastore、auth、metrics、health、routes 和 HTTP server。
3. `fx.New` 同步构建依赖图、执行 invoke 及其 constructor 依赖；该阶段不受 `runtime.lifecycle.start_timeout` 限制。
4. CLI 使用同一配置值建立显式 Start context 并调用 `App.Start`，该 context 约束全部 `OnStart` hooks；业务 HTTP 与启用的 pprof 必须先同步绑定 listener，绑定失败直接阻断启动，成功后才异步 Serve。
5. 收到外部终止信号或 HTTP/pprof 异常退出产生的 Fx shutdown signal 后，CLI 使用同一配置值建立显式 Stop context 并调用一次 `App.Stop`；各 Managed 在独立内部 shutdown timeout 内继续后台 cleanup，Fx context 只约束本次 hook 等待和 App 总预算。

### 6.2 登录和会话

1. HTTP 请求进入 `auth/transport/http` controller。
2. application command 校验输入、凭证和用户状态。
3. session store 写入 Redis 会话状态。
4. user-service 私有 token issuer 签发 access token 和 refresh token。
5. 响应通过共享 response helper 返回统一 envelope。
6. 强制改密场景下，数据库事务原子更新密码、重置 `must_change_password` 标记并自增 `token_version`；旧会话清理为尽力而为（best-effort）异步降级保障，若 Redis 短暂不可用，记录 WARN 日志而不阻断主事务，旧 Access Token 立即通过自增的 `token_version` 被鉴权层阻断，避免用户重试死锁。

### 6.3 受保护 API 授权

1. 请求进入 `/api/v1` authenticated group。
2. 共享 HTTP auth middleware 调用最小 access token verifier 接口，user-service verifier 校验 JWT、access subject、认证 claims 和 token version。
3. RBAC 中间件读取当前用户和请求资源。
4. permission authorizer 使用 Casbin 或同步后的 policy 判断访问权限。
5. 通过后进入目标 controller。

在线 RBAC policy 写入提交后，Redis Pub/Sub 只向各副本发送快速唤醒 hint，PostgreSQL latest policy revision 是 Casbin 静态授权规则恢复与收敛的唯一权威事实。在线写操作的 Pub/Sub 广播采用尽力而为（best-effort）语义，广播失败记录 WARN 日志并由权威数据库 revision 周期对账收敛，不制造系统级 ERROR 虚假告警。用户角色绑定写入使用独立 user-role revision，只驱动用户角色缓存失效，不推进 Casbin engine applied revision，也不触发 `role_permissions` 全量加载。`platform/runtime/redispubsub` 独立监督可重建的 Pub/Sub subscription，permission Redis watcher 只消费其消息并在自己的根生命周期中执行启动立即校准和后续周期数据库 policy revision 校准；通用 subscriber 的确认、Receive 或协议失败只进入有界退避重连，不停止 watcher 的权威校准。消息处理与周期校准在 watcher 根循环中串行执行，并合并同批待处理通知，防止 Casbin projection 并发倒退或被重复消息放大全量重建。Casbin `LoadPoliciesAtLeast` 在等待超前 revision 时具备最大重试上限（默认 5 次）与指数退避，超时后明确返回错误交由定时权威校准兜底，消除长生命周期单飞重载中的无休止轮询。readiness 依据最后一次完整权威校准的 staleness 判定，当前订阅正在重连但校准仍新鲜时不制造 watcher 粘滞失败。

### 6.4 RBAC seed 和超级管理员 bootstrap

1. `rbac seed` 加载 user-service 私有配置，按服务私有资源名打开 user DB，创建 Ent client。
2. role seed service 按 `rbacbaseline.DefaultPermissions()` 的稳定 `permission_id` 创建或更新权限投影，并维护系统角色和默认绑定。
3. 权限定义变更随代码发布；路由测试构建真实 Gin route graph，与代码基线双向比较，missing 或 stale 均阻断 CI。
4. `bootstrap-super-admin` 读取 `ADMIN_BOOTSTRAP_PASSWORD`，使用 `rbacbaseline.BootstrapSuperAdminUserID` 创建 `MustChangePassword` 用户并绑定内置超级管理员角色。
5. 后续超级管理员授权通过在线用户角色绑定 API 完成，由在线流程负责 user-role revision 发布和用户角色缓存失效；只有角色状态、角色权限或权限投影变化才负责 policy revision 发布和 Casbin policy 收敛。

系统内置 RBAC 角色、权限和 bootstrap 用户 ID 由 `user-service/internal/shared/rbacbaseline/ids.go` 统一定义为手写固化 UUID 字符串，semantic name 绑定稳定业务授权语义，不绑定项目展示名、HTTP path、中文文案或 Go symbol。普通运行时业务实体仍使用 `platform/runtime/id.NewUUID()` 生成 UUID v7；已有项目重命名不得默认修改、重算或复用系统内置 ID。

### 6.5 数据迁移

1. Ent schema 变化后执行 `make user-service-generate`。
2. 使用 `make user-service-migrate-diff name=<migration-name>` 生成 Atlas migration。
3. 使用 `make user-service-migrate-validate` 校验 migration。
4. 使用 `atlas migrate hash` 或等价流程刷新 `atlas.sum`，将 SQL migration 与权限要求提交 Git。
5. `users.nickname` substring 模糊查询统一使用 `pg_trgm` 提供的 GIN `gin_trgm_ops` 索引，不保留普通索引、无扩展 fallback 或双索引兼容分支；发布时通过 DBA 工单或受控发布平台人工或受控执行 SQL migration，并确认 `CREATE EXTENSION IF NOT EXISTS pg_trgm;` 所需的 DBA 权限或前置动作。
6. 删除权限时 migration 先删除 `role_permissions` 再删除 `permissions`；随后执行同版本 RBAC seed，并通过显式 reload 或滚动重启收敛 Casbin policy。

## 7. 部署和观测

共享核心配置只含 `app/runtime/server/log/observability`。Redis/PostgreSQL 类型由 `platform/runtime/resources` 提供，user-service 在 `resources.redis.cache_redis` 与 `resources.postgres.primary_db` 声明实际资源；Redis 通过 `mode` 选择 `cluster` 或 `standalone`，集群使用 `addrs`，非集群使用 `addr`，两种 mode 均固定使用 Redis 0 号库且不暴露 `db` 配置。feature cache 由 `auth.token_version_cache` 和 `rbac.user_role_cache` 各自拥有启停与 TTL；容量和回源超时由 user-service 代码常量提供，不进入配置文件。日志输出到 stdout/stderr，tracing 启用后固定使用 OTLP，进程时区由已校验的 `runtime.timezone` 配置控制，不读取平台 `TZ`。

- Dockerfile：`deployments/docker/user-service.Dockerfile` 使用 BuildKit manifest-first 依赖层、只读 Go module 解析、静态编译和固定 digest 的 `gcr.io/distroless/static-debian12:nonroot` 运行时；运行镜像身份为 UID/GID `65532`，不包含 shell、包管理器、下载工具或 Atlas。
- Compose：`deployments/compose/docker-compose.yml` 继承 Distroless `nonroot` 身份，user-service healthcheck 使用 exec-form 调用原生 `healthcheck` CLI。
- 本地 Nacos：`deployments/nacos/` 是 Git 权威来源；两个 Compose 初始化任务分别将 `local-host/` 与 `local-docker/` 的完整三文档发布到 `loca-host` 与 `loca-docker`。
- Helm：`deployments/helm/aegiscore-user-service/` 是唯一 Kubernetes 生产清单来源，渲染 UID/GID/fsGroup `65532`、只读根文件系统、`/tmp` emptyDir、kubelet HTTP probes 和 RBAC seed Job，并要求生产发布显式传入不可变 `image.ref`。不要再维护手写原生 Kubernetes YAML。
- CI：主 `ci.yml` 唯一触发仅支持 `workflow_call` 的质量 workflow，使同一 commit 的 lint 与普通单测各运行一次；独立 `container-test` job 通过根 `make test-containers` 显式运行 platform 与 user-service 的真实 PostgreSQL/Redis 测试；镜像安全 job 复用同一 BuildKit image ID 执行内容断言、Trivy HIGH/CRITICAL 门禁和 SBOM 生成。
- Prometheus alerts：`deployments/observability/prometheus/user-service-alerts.yaml`。
- Grafana dashboard：`deployments/observability/grafana/user-service-overview.json`。
- Compose dashboard 由 `deployments/compose/scripts/generate-grafana-dashboard.sh` 从通用观测 dashboard 生成。

## 8. 规格边界

稳定能力位于 `openspec/specs/`。跨 feature、跨模块、外部契约、schema、部署或行为变更必须先创建 `openspec/changes/<change-name>/`，完成后把 delta 合并回主规格并删除 change 目录。仓库不保留 `openspec/changes/archive/`。
