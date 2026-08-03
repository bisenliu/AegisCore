## Purpose

定义 AegisCore 的交付运维能力，覆盖构建、运行、测试、lint、生成、数据库迁移、容器、部署资产和发布顺序。
## Requirements
### Requirement: 构建、运行与稳定 CLI 入口

系统 MUST 通过统一 Makefile 和 user-service CLI 提供可重复的构建、运行及进程生命周期控制。公开命令、flag、退出码和错误传播属于运维契约，变更时 MUST 通过对应 capability 明确迁移。唯一公开 CLI 根命令 MUST 为 `aegiscore-user-service`，MUST NOT 保留旧 `aegiscore-user-services` 别名、隐藏命令或兼容入口。

#### Scenario: 构建、帮助与 RBAC 运维入口

- **WHEN** 执行 `make build` 或 `make user-service-run`
- **THEN** 系统 MUST 分别将二进制构建到 `USER_SERVICE_BIN`，或使用 `USER_SERVICE_CONFIG` 启动 `aegiscore-user-service serve`
- **WHEN** 执行根或 user-service help
- **THEN** 系统 MUST 输出中文命令说明，并保持 `serve`、`rbac`、`fxgraph` 的名称、公开 flag、默认配置路径、退出码和输出语义稳定
- **AND** RBAC help MUST 只展示 `seed` 和 `bootstrap-super-admin`，help、测试、文档和 Makefile MUST NOT 展示旧根命令、`rbac create-super-admin`、`rbac assign-super-admin`、`--reset-password`、`ADMIN_RESET_PASSWORD` 或旧超级管理员别名
- **WHEN** 运维执行带 `ADMIN_BOOTSTRAP_PASSWORD`、`ADMIN_USERNAME` 和 `ADMIN_NICKNAME` 的 `make user-service-bootstrap-super-admin`
- **THEN** Makefile MUST 调用 `aegiscore-user-service rbac bootstrap-super-admin --username "$ADMIN_USERNAME" --nickname "$ADMIN_NICKNAME" --password-env ADMIN_BOOTSTRAP_PASSWORD`
- **AND** 根 Makefile MUST 仅提供带服务名前缀的目标，MUST NOT 展开、打印或记录密码，也 MUST NOT 提供 `bootstrap-super-admin`、`user-service-create-super-admin`、`create-super-admin` 或 `ADMIN_RESET_PASSWORD` 入口

#### Scenario: 外部与内部退出协调

- **WHEN** 上游 context 取消或 `App.Wait()` 返回 shutdown signal
- **THEN** serve 命令 MUST 使用未被取消的上游 context value 和配置化预算且仅调用一次 `App.Stop()`
- **AND** 非零内部 exit code 或 Stop error MUST 转换为保留全部诊断信息的 Cobra error，命令内部 MUST NOT 调用 `os.Exit`

### Requirement: 质量门禁、架构诊断与可复现生成

系统 MUST 提供模块级和仓库级测试、lint、架构检查、生成和完整 verify 入口。测试与生成 MUST 可诊断、可复现且不得扩张正式生产 API。`user-service-architecture-lint` MUST 保护正式架构来源声明的边界；Fx 依赖图诊断 MUST 无外部及运行时激活副作用。OpenAPI 转换库、CLI 和服务脚本 MUST 分别由 `common/http/openapi`、`tools/openapi-convert` 和 user-service 拥有其通用逻辑、可执行入口与服务参数。

#### Scenario: 统一质量与生成门禁

- **WHEN** 执行 `make test`、`make lint` 或 `make verify`
- **THEN** 系统 MUST 分别运行 common 与 user-service 测试、各 module 的 `golangci-lint`，或覆盖 lint、架构检查、测试、必要生成并以 `git diff --exit-code` 检测 drift
- **WHEN** CI 检查 user-service 架构、OpenAPI 或 migration
- **THEN** MUST 使用 `make user-service-architecture-lint`、`make user-service-openapi-generate` 和 `make user-service-migrate-validate`，MUST NOT 调用不存在或缺少服务前缀的私有目标
- **WHEN** package 需要 mock、metrics no-op 或其他 Go 生成物
- **THEN** module MUST 显式声明工具依赖，生成入口 MUST 归消费 package 所有且排除正常构建，生成物 MUST 位于测试边界或所属 feature；`make common-generate`、`make user-service-generate` 和 verify MUST 可重建并检测 drift
- **WHEN** 非测试 Go 文件引入仅测试消费、暴露内部状态、全局可变替身或等价测试专用 API
- **THEN** 架构检查 MUST 拒绝，测试 MUST 使用现有实现、局部依赖注入、消费侧最小接口或 `common/testing`，MUST NOT 驱动冗余生产分支和适配层
- **WHEN** 测试构造 Fx app、feature module、启动失败或 rollback，或直接调用可能阻塞的停止函数
- **THEN** 测试 MUST 保留可定位诊断信息，并使用带 timeout 的 context 和测试级 guard；实现不尊重 context 时 MUST 在 guard 内失败，MUST NOT 退化为等待全局 `go test -timeout`

#### Scenario: 架构边界与 Fx graph

- **WHEN** 业务代码违反 feature-first、分层、共享边界、生成配置或部署契约
- **THEN** `user-service-architecture-lint` MUST 失败并指向正式架构来源；当前不存在的 gRPC、MQ、eventbus 或 outbox MUST NOT 以空壳或推测性实现进入正式边界
- **WHEN** 执行 `cd user-service && go run ./cmd fxgraph --config ./configs/config.yaml --output /tmp/aegis-fx.dot`
- **THEN** 系统 MUST 基于正式配置投影和无运行时激活的 wiring graph 或专用 graph root 生成非空 DOT
- **AND** 生成过程 MUST NOT 执行生产 `fx.Invoke`、连接 PostgreSQL/Redis/OTLP、启动 listener、创建 workerpool/localcache/tracing exporter 后台资源、注册真实 route 或 runtime metrics，也 MUST NOT 修改 `TZ`、`time.Local` 或 Gin mode
- **WHEN** `serve` 构建正式 Fx App
- **THEN** 系统 MUST 使用同时包含 wiring 与 runtime module 的正式 App module，并保持 HTTP、pprof、route、dependency metrics、timezone、RBAC lifecycle 和 hooks 的运行时语义；graph root MUST NOT 取代该激活链路

#### Scenario: OpenAPI 转换、生成与 drift

- **WHEN** 执行 `make user-service-openapi-generate`
- **THEN** user-service 脚本 MUST 调用仓库级 CLI 将 Swagger 转换为 OpenAPI 3，并更新 `openapi.go`、`openapi.json` 和 `openapi.yaml`
- **WHEN** 生成需要 server、探活路径、security scheme 或输出路径
- **THEN** user-service 脚本 MUST 显式传参，通用 CLI MUST NOT 写死 `/api/v1`、健康路由或 `BearerAuth` 等服务语义
- **WHEN** 转换工具或依赖变化
- **THEN** 测试 MUST 验证结构化输出和错误路径，完整验证 MUST 检测生成物及依赖 drift

### Requirement: Ent/Atlas migration 交付

Ent schema MUST 是数据库结构来源，Atlas SQL migration MUST 是可审查交付工件。user-service 的 Ent schema、client、entity、predicate、`enttest` 和 `migrate` 生成物 MUST 位于 `user-service/internal/persistence/ent/`，并通过 Go `internal` 规则作为 user-service 私有持久化实现受保护。仓库 MUST 支持生成、diff、validate 和 hash 校验，但 MUST NOT 提供自动连接目标数据库执行 `atlas migrate apply` 的入口，也 MUST NOT 保留模块根级 `github.com/aegiscore/user-service/ent` 兼容包、别名、shim 或双路径支持。

#### Scenario: Schema、migration 与数据库结构

- **WHEN** Ent schema 或生成特性变化
- **THEN** 协作者 MUST 执行 `make user-service-generate` 和 `make user-service-migrate-diff name=<migration-name>` 并审查 SQL 与 `atlas.sum`；interface、生成指令、Ent 生成物、SQL 或 hash 不一致时验证 MUST 失败
- **WHEN** 生成或审查 SQL migration
- **THEN** migration MUST NOT 包含 `FOREIGN KEY` 或 `REFERENCES`，并 MUST 保留 Ent edge、关联字段和必要唯一索引
- **WHEN** migration 使用 `gin_trgm_ops`
- **THEN** 首个 migration MUST 在索引前创建 `pg_trgm` 并提示 DBA 权限
- **AND** Atlas dev Dockerfile、diff 脚本、`atlas.hcl` 与 Compose 本地 image tag MUST 一致，lint MUST 检测 drift

#### Scenario: Migration 校验与受控执行

- **WHEN** migration 准备发布或 SQL 经手工调整
- **THEN** `make user-service-migrate-validate` MUST 校验 SQL 和 `atlas.sum`，手工调整后 MUST 刷新 hash 并重新验证
- **AND** SQL MUST 提交 Git，并通过 DBA 工单或受控发布平台执行
- **AND** user-service、E2E、Makefile、脚本和部署资产准备数据库时 MUST 使用已提交 migration，运行时代码 MUST NOT 使用 `client.Schema.Create(ctx)` 表达 schema 变更
- **AND** 仓库 MUST NOT 提供 `migrate-apply`、自动 migration Job 或等价 Atlas apply 入口

#### Scenario: Ent 生成包路径受 internal 保护

- **WHEN** 执行 `make user-service-generate` 或 Ent 生成入口
- **THEN** Ent 生成物 MUST 收敛到 `github.com/aegiscore/user-service/internal/persistence/ent` 及其子包
- **AND** 生成流程 MUST NOT 创建、更新或依赖 `user-service/ent/` 根级目录
- **WHEN** user-service 内部 provider、feature infrastructure、RBAC CLI、Atlas schema helper 或测试需要访问 Ent client、entity、predicate、`enttest` 或 `migrate`
- **THEN** 代码 MUST 导入 `github.com/aegiscore/user-service/internal/persistence/ent` 及其子包
- **AND** 代码 MUST NOT 导入 `github.com/aegiscore/user-service/ent` 及其子包
- **WHEN** 其他 workspace module 或未来服务尝试直接 import user-service 的 Ent 包
- **THEN** Go `internal` 规则 MUST 阻止该导入，调用方 MUST 通过正式服务边界访问 user-service 能力

### Requirement: 镜像、部署与受控发布

user-service 镜像 MUST 使用 BuildKit、不可变基础镜像、只读 Go module 解析和分离缓存构建，并以最小非 root 运行时交付同一可验证工件。CI MUST 运行真实依赖测试。Docker、Compose、Kubernetes、Helm 和观测资产 MUST 使用一致的 runtime config、release 工件、安全上下文、探针、Secret、资源和网络边界，MUST NOT 包含真实 secret 或自动 migration apply。外部服务标识与镜像名 MUST 使用 `aegiscore-user-service`，容器、二进制和本地目录语义 MUST 使用 `user-service`，MUST NOT 保留旧复数命名。

#### Scenario: 可复现且最小的镜像工件

- **WHEN** Docker 构建准备依赖和编译
- **THEN** MUST 在源码前复制 `go.work`、workspace module manifests 和已有 checksum 文件，使用持久化 module/build cache、`-mod=readonly`、`-trimpath` 和显式 VCS metadata 策略，MUST NOT 修改 `go.work.sum`、`go.mod` 或 `go.sum`
- **AND** 构建输出和 ENTRYPOINT MUST 指向 `/app/user-service/bin/user-service`
- **WHEN** Dockerfile 声明基础镜像
- **THEN** builder 与 runtime image MUST 同时保留可读 tag 和审核 digest；CI 内容断言、漏洞扫描和 SBOM MUST 复用同一 image ID 或 digest，MUST NOT 分别冷构建不同工件，默认 `USER_SERVICE_IMAGE` MUST 为 `aegiscore-user-service:<sha>`
- **WHEN** 容器启动
- **THEN** 镜像 MUST 以非 root 用户运行，仅包含运行必需内容、证书和时区数据，MUST NOT 包含 Go toolchain、shell、Atlas、migration SQL 或默认 migration apply 入口
- **AND** 本地或 Compose OCI healthcheck MUST 以镜像内探针或等价无 shell 机制访问 `/livez`；Kubernetes MUST 使用原生 `httpGet` probes，MUST NOT 依赖容器 healthcheck 命令

#### Scenario: CI 真实依赖测试

- **WHEN** PR 或主线 push 执行阻塞式 test job
- **THEN** job MUST 设置唯一开关 `AEGISCORE_TEST_CONTAINERS=1` 并运行 `make test`，MUST NOT 读取 `TEST_CONTAINERS` 或仅以 `AEGISCORE_TEST_E2E` 替代完整门禁
- **WHEN** 该开关启用
- **THEN** Docker daemon、镜像、容器、migration 或配置前置失败 MUST 使 job 失败，PostgreSQL/Redis smoke、role store 和 HTTP E2E MUST 实际执行而非 skip
- **AND** E2E harness MUST 使用当前严格配置、真实 PostgreSQL/Redis 和已提交 migration，覆盖认证与用户 HTTP flow；verify、race 或 coverage job MAY 不重复该容器负载

#### Scenario: 部署资产与安全基线

- **WHEN** 启动 Compose
- **THEN** 系统 MUST 提供数据库、缓存和观测服务并使用当前 runtime config 与 release 工件；SQL migration MUST 在 seed 前完成，seed MUST 在 app 前完成，Compose MUST NOT 自动执行 `atlas migrate apply`
- **AND** Compose 镜像、healthcheck、Prometheus scrape label 和脚本 MUST 使用当前单数命名或 `/app/user-service/bin/user-service`
- **WHEN** 渲染或部署原生 Kubernetes 清单或 Helm chart
- **THEN** Pod MUST 非 root、只读根文件系统、禁止特权升级、收敛 capabilities，并设置 CPU/内存 requests 与 limits
- **AND** liveness、readiness、startup MUST 分别访问 `/livez`、`/readyz`、`/startupz`；Secret MUST 外部注入，默认配置和值 MUST NOT 包含真实 secret
- **AND** Deployment 和 chart MUST NOT 设置 `RUN_MIGRATIONS=true`、渲染 Atlas migration Job、复制 migration SQL 或依赖运行时镜像内 Atlas；Deployment、Service、ServiceAccount、ConfigMap、Secret、Job、PDB、HPA、NetworkPolicy、Helm chart 和 release 示例 MUST 使用当前单数服务名
- **WHEN** NetworkPolicy 允许 ingress 或访问 PostgreSQL、Redis、OTLP
- **THEN** ingress MUST 使用明确 namespace 与 pod selector，egress MUST 使用目标 selector 或精确 `ipBlock`，MUST NOT 以空 namespace selector 或仅按端口放行任意目的地；准入标签 MUST 由 admission policy 或等价控制限制
- **WHEN** Kubernetes 资产或 chart 变化
- **THEN** README 或 tasks MUST 提供 YAML/schema、server-side dry-run、`helm lint` 或 `helm template` 等验证命令，并验证安全上下文、探针、资源、seed、NetworkPolicy、无 migration Job 和无旧复数命名 drift

#### Scenario: 发布顺序与兼容边界

- **WHEN** 在全新数据库发布 user-service
- **THEN** 运维 MUST 先确认已提交 migration 经 DBA 工单或受控平台执行，再运行 RBAC seed、`rbac bootstrap-super-admin`，最后启动或滚动 HTTP 副本；任一前置步骤失败 MUST 阻止 rollout
- **AND** 初始管理员 MUST 在 HTTP 副本启动后通过强制改密设置正式密码，seed 与 bootstrap Job MUST 使用和 Deployment 相同的当前发布镜像与工件集合
- **WHEN** 发布包含 `rbac bootstrap-super-admin` 的版本
- **THEN** 系统 MUST 只支持全新数据库路径，MUST NOT 支持旧库原地升级、旧管理员识别、bootstrap marker 回填、旧命令别名、双版本 CLI 或自动恢复已有管理员，也 MUST NOT 新增 Ent schema、业务表或 Atlas migration
- **WHEN** 普通运行时容器启动
- **THEN** 容器 MUST 直接执行服务或显式 CLI，`RUN_MIGRATIONS=true` MUST NOT 触发 Atlas migration

#### Scenario: 当前配置与优雅终止

- **WHEN** 交付 user-service 配置
- **THEN** server MUST 使用 `server.http` 和默认禁用的 `server.grpc`，资源 MUST 使用 `resources.redis` 与 `resources.postgres`，主 PostgreSQL 路径 MUST 为 `resources.postgres.primary_db`
- **AND** 环境变量 MUST 使用当前嵌套路径，时区 MUST 使用 `TZ`，secret MUST 通过环境变量或 Secret 注入
- **WHEN** lifecycle timeout、preStop、原生清单或 Helm values 变化
- **THEN** 结构化测试 MUST 验证 termination grace 不小于 `runtime.lifecycle.stop_timeout` 加平台安全余量，且原生 Kubernetes 与 Helm 默认值一致，MUST NOT 以正则误匹配注释或无关字段
- **WHEN** 滚动发布、缩容、驱逐或故障退出终止 Pod
- **THEN** kubelet MUST 为完整 Fx Stop 链路与平台阶段保留默认宽限期；正常关闭后 MUST 立即退出，仅在宽限期耗尽且进程未退出时强制终止

### Requirement: 项目身份初始化与重命名 ID 边界

系统 MUST 区分从基础框架初始化新项目和已有项目重命名。新项目初始化 MAY 写入新的 RBAC 系统 ID 固化常量；已有项目重命名 MUST NOT 默认修改、重算或复用系统内置 RBAC、permission 或 bootstrap 用户 ID。

#### Scenario: 初始化、重命名与文档边界

- **WHEN** AegisCore 作为基础框架复制为全新项目
- **THEN** 初始化流程 MAY 将新的手写固化 UUID 字符串常量写入 `user-service/internal/shared/rbacbaseline/ids.go`
- **AND** 初始化 MUST 只修改代码或文档，MUST NOT 连接或修改数据库，也 MUST NOT 执行 RBAC seed
- **WHEN** 已有项目修改展示名、服务名、module path、CLI、镜像、部署资源或观测 label
- **THEN** 重命名 MUST NOT 默认修改 `SuperAdminRoleID`、`BootstrapSuperAdminUserID`、baseline permission ID 或数据库中的角色、权限、用户和绑定 ID
- **AND** 重算系统 ID MUST 作为单独高风险数据迁移 change，MUST NOT 混入普通重命名
- **WHEN** 仓库提供初始化或重命名脚本
- **THEN** 文档 MUST 明确初始化仅用于新项目且重命名默认不重算系统 ID，脚本或 README MUST NOT 宣称已有项目改名会自动迁移 RBAC 系统 ID

### Requirement: 本地 Compose 时区一致性

本地 Compose 的常驻服务与一次性任务 MUST 显式使用 `TZ=Asia/Shanghai`，不得依赖宿主机时区或镜像浮动默认值。缺少 IANA zoneinfo 的基础镜像 MUST 通过可审查的最小镜像层补齐所需时区数据，不得保留设置了 `TZ` 但进程仍以 UTC 运行的无效配置。

#### Scenario: 全部 Compose 服务声明时区

- **WHEN** 渲染 `deployments/compose/docker-compose.yml`
- **THEN** PostgreSQL、Redis、Nacos、Nacos 初始化任务、Jaeger、RBAC seed、user-service、Prometheus 和 Grafana MUST 都获得 `TZ=Asia/Shanghai`
- **AND** 配置 MUST NOT 挂载宿主机 `/etc/localtime` 或把宿主机时区作为正确性前提

#### Scenario: Jaeger 基础镜像缺少 zoneinfo

- **WHEN** 当前 Jaeger 基础镜像不包含 `/usr/share/zoneinfo/Asia/Shanghai`
- **THEN** 本地 Jaeger 镜像 MUST 从固定且可审查的构建阶段复制该 IANA zoneinfo，并在进程启动后解析为 UTC+8
- **AND** 薄镜像 MUST 保留基础 Jaeger 的 entrypoint、端口与功能，MUST NOT 为时区修复引入 Jaeger 版本迁移

#### Scenario: PostgreSQL 已有数据卷

- **WHEN** Compose 使用已经初始化且配置为 UTC 的 PostgreSQL data volume 重建容器
- **THEN** PostgreSQL session `timezone` 与 `log_timezone` MUST 在不删除 volume、不运行 migration 的情况下变为 `Asia/Shanghai`
- **AND** 官方 `docker-entrypoint.sh` 与健康检查 MUST 继续正常工作

### Requirement: Compose 本地编排

系统 MUST 提供可从仓库根目录运行的本地 Compose 编排，启动 PostgreSQL、Redis、Jaeger OTLP、本地用户服务、Prometheus 和 Grafana。Compose MUST 使用本地必填环境变量注入数据库、JWT 和 Grafana 密码，MUST 在缺少必填值时提前失败。Compose 默认 MUST 只发布当前真实本地入口端口，MUST NOT 默认发布没有真实入站 API 的 gRPC 端口或默认关闭的诊断端口。

#### Scenario: 默认本地入口端口

- **WHEN** 调用方使用必填本地环境变量渲染 `deployments/compose/docker-compose.yml`
- **THEN** Compose MUST 发布 user-service HTTP `8080:8080`
- **AND** Compose MUST 发布 PostgreSQL、Redis、Jaeger、Prometheus 和 Grafana 的本地入口端口
- **AND** Compose MUST NOT 发布 `19090:9090`
- **AND** Compose MUST NOT 发布 `6060:6060`

#### Scenario: 当前无真实入站 gRPC API

- **WHEN** user-service 尚未提供真实入站 gRPC API
- **THEN** Compose 默认 MUST 设置 `AEGISCORE_SERVER_GRPC_ENABLED=false`
- **AND** Compose MUST NOT 为 user-service 发布宿主 gRPC 端口

### Requirement: 独立本地 Nacos 配置目录与双初始化服务

系统 MUST 为主机和 Docker 分别保存完整、可独立查看、校验和发布的 Nacos 配置目录，并通过两个 Compose 一次性服务分别调用单目录、单 Namespace seed 工具发布到独立 Namespace。每个目录 MUST 完整包含固定三 dataId；系统 MUST 接受少量公共字段重复，并通过结构化测试约束除明确环境字段外的配置一致性。

#### Scenario: 两套完整配置目录

- **WHEN** 仓库保存本地主机和 Docker 的 Nacos 配置
- **THEN** `deployments/nacos/local-host/` 和 `deployments/nacos/local-docker/` MUST 各自完整包含 `base.yaml`、`resources.yaml`、`user-service.yaml`
- **AND** 任一目录 MUST 能在不读取另一目录、公共配置目录、overlay、target manifest 或第四文档的情况下独立发布和加载
- **AND** 主机目录 MUST 直接声明宿主机地址与映射端口，Docker 目录 MUST 直接声明 Compose DNS 与容器端口
- **AND** 旧 `deployments/compose/nacos/init/` MUST NOT 继续作为第二权威配置来源

#### Scenario: 两个单目标初始化服务

- **WHEN** Compose 启动本地 Nacos 配置初始化
- **THEN** `nacos-init-host` MUST 只读挂载 `deployments/nacos/local-host/` 并发布到 `loca-host`
- **AND** `nacos-init-docker` MUST 只读挂载 `deployments/nacos/local-docker/` 并发布到 `loca-docker`
- **AND** 两个服务 MUST 使用 `AEGISCORE` group 和 `base.yaml`、`resources.yaml`、`user-service.yaml` 三 dataId
- **AND** Docker user-service 与 RBAC seed MUST 在 `nacos-init-docker` 成功后启动，MUST NOT 依赖主机配置完成发布

#### Scenario: seed 工具保持单目录单 Namespace 契约

- **WHEN** 任一 Nacos 初始化服务调用 `nacos-config-seed`
- **THEN** 工具 MUST 继续通过 `--config-dir`、`--namespace`、`--group`、`--data-ids` 或对应环境变量描述单次发布
- **AND** 工具 MUST NOT 提供 `--config-root`、`--targets`、目录扫描或单进程多 Namespace 编排入口
- **AND** 工具 MUST 在网络写入前读取并校验当前目录的全部声明文档，再创建或复用单个 Namespace 并按 dataId 顺序覆盖发布
- **AND** 相同输入重复执行 MUST 幂等收敛；失败 MUST 返回包含 Namespace、group 和 dataId 的诊断，MUST NOT 自动删除已经发布的配置

#### Scenario: 完整配置防漂移

- **WHEN** CI 或本地验证比较 `local-host` 与 `local-docker` 配置
- **THEN** 测试 MUST 结构化解析同名 YAML 文档，并在排除精确声明的环境字段路径后要求其余配置相等
- **AND** 允许差异 MUST 仅覆盖主机名、端口、OTLP endpoint 或经审查确认依赖运行位置的叶子字段，MUST NOT 使用顶层 section 或通配路径跳过比较
- **AND** 两个目录的文件集合 MUST 恰好为固定三 dataId，每套文档 MUST 分别通过 user-service strict decode、normalize 和 validate

#### Scenario: 主机与 Compose 选择运行时来源

- **WHEN** user-service 或 RBAC seed 在 Compose 网络内运行
- **THEN** workload MUST 使用 `AEGISCORE_NACOS_NAMESPACE=loca-docker`，并按 `base.yaml`、`resources.yaml`、`user-service.yaml` 加载
- **AND** effective 配置 MUST 使用 Compose DNS 和容器端口访问 PostgreSQL、Redis 与 OTLP
- **WHEN** user-service 在主机直接运行并复用 Compose 依赖
- **THEN** 进程 MUST 使用 `AEGISCORE_NACOS_NAMESPACE=loca-host` 和相同三 dataId 顺序
- **AND** effective 配置 MUST 使用宿主机地址和 Compose 映射端口访问 PostgreSQL、Redis 与 OTLP

#### Scenario: 敏感配置边界

- **WHEN** 提交任一本地完整配置目录
- **THEN** 文件 MUST NOT 包含 Nacos 凭据、真实 JWT secret、真实 PostgreSQL/Redis 密码或其他 production-like secret
- **AND** Nacos 发布认证 MUST 仅通过进程环境或 Secret 注入，seed 工具 MUST NOT 把凭据或配置文档内容写入日志
- **AND** 固定本地开发占位值 MUST 明确限制为 local 使用且 MUST NOT 被 production-like 部署引用

#### Scenario: 从单一 loca Namespace 迁移

- **WHEN** 本地环境从旧 `loca` Namespace 和平铺配置迁移
- **THEN** 运维 MUST 先发布并验证 `loca-host` 与 `loca-docker`，再切换 Compose 和主机运行命令，最后移除 Git 中的旧平铺来源
- **AND** 迁移验证 MUST 分别检查两个 Namespace 的配置来源、严格校验、脱敏 effective render 和依赖地址
- **AND** seed 工具 MUST NOT 自动删除旧 `loca` Namespace；回滚 MAY 在旧内容仍保留时恢复旧来源选择，旧 Namespace 的最终清理由运维显式执行

### Requirement: Redis Cluster 配置交付

Nacos、Compose、Kubernetes、Helm、README、E2E harness 和测试配置 fixture MUST 使用 Redis mode-driven 配置契约。交付资产 MUST 明确展示 `mode: cluster` 或 `mode: standalone`，MUST NOT 使用隐式 Redis mode、Redis DB 配置或 Sentinel 参数。

#### Scenario: Nacos 与部署配置使用 Cluster 契约

- **WHEN** 渲染或加载 user-service 运行时配置
- **THEN** Redis 资源 MUST 使用 `resources.redis.cache_redis.mode` 选择 `cluster` 或 `standalone`
- **AND** `addrs` MUST 允许单个阿里云 Redis 集群访问地址作为 seed endpoint
- **AND** Cluster 示例 MUST 使用 `addrs`，standalone 示例 MUST 使用 `addr`，配置示例 MUST NOT 展示 Redis `db` 或 Sentinel 字段

#### Scenario: Compose 与真实依赖测试

- **WHEN** Compose 或 Docker-backed 测试需要 Redis
- **THEN** 资产 MUST 提供 Redis Cluster fixture 或明确连接外部 Redis Cluster 的配置路径
- **AND** `AEGISCORE_TEST_CONTAINERS=1` 下的 Redis Cluster 兼容测试 MUST 覆盖 auth、RBAC、health 和 metrics 的 Cluster-sensitive 行为

### Requirement: Redis Cluster 发布与回滚

Redis Cluster 发布 MUST 以空 Cluster 和新配置切换为前提，不迁移旧 Redis 数据。回滚 MUST 同步回滚应用镜像和 Redis 配置契约，不要求从 Redis Cluster 回写旧 Redis。

#### Scenario: 发布顺序

- **WHEN** 发布 Redis Cluster 支持版本
- **THEN** 运维 MUST 先准备 Redis Cluster 并更新 Nacos、Kubernetes 或 Helm Redis 配置，再滚动发布 user-service
- **AND** 发布验证 MUST 覆盖 `/readyz`、`/startupz`、Redis metrics、登录、refresh、退出全部会话、强制改密和 RBAC 写后同步

#### Scenario: 回滚边界

- **WHEN** 需要回滚到旧 Redis 单机版本
- **THEN** 运维 MUST 同步回滚应用镜像和 Redis 配置为旧版本要求的契约
- **AND** 系统 MUST 接受 refresh session、password-change session、token version cache 和 RBAC policy version 在回滚过程中失效或重建

### Requirement: Trusted proxy 配置交付

Nacos、Compose、Kubernetes、Helm、README 和部署说明 MUST 使用 `server.http.trusted_proxies` 表达 user-service 受信任入口代理 IP 或 CIDR。生产和 production-like 环境 MUST 按实际 Ingress、gateway、ALB、Envoy、Nginx 或 service mesh 拓扑配置该列表；系统 MUST NOT 提供旧 `http.trusted_proxies` 示例或兼容说明。

#### Scenario: 部署配置声明可信入口

- **WHEN** user-service 部署在反向代理、Ingress、gateway、load balancer 或 service mesh 后方
- **THEN** 运行配置 MUST 在 `server.http.trusted_proxies` 中声明真实入口代理 IP 或 CIDR
- **AND** 配置示例 MUST 明确该列表需要按环境拓扑填写，MUST NOT 默认信任所有私有网段或所有来源

#### Scenario: 入口层清洗 forwarded headers

- **WHEN** 外部请求进入 Ingress、gateway、ALB、Envoy、Nginx 或 service mesh 边界
- **THEN** 入口层 MUST 覆盖或重建 `X-Forwarded-For` 和 `X-Real-IP` 等 forwarded headers
- **AND** 入口层 MUST NOT 将客户端提供的未清洗 forwarded headers 直接透传给 user-service

#### Scenario: 发布验证客户端地址

- **WHEN** 发布启用 trusted proxy 配置的 user-service
- **THEN** 发布验证 MUST 覆盖登录请求在受信任代理后方记录真实客户端地址
- **AND** 未配置或错误配置 trusted proxy 时，验证 MUST 能暴露 `client_ip` 仍为代理地址的 drift

### Requirement: Helm 生产镜像不可变发布

user-service Helm 生产发布 MUST 使用不可变镜像引用。Deployment、RBAC seed Job、安全扫描、SBOM、镜像身份记录和 Helm release MUST 指向同一个已构建并已推送的 user-service 镜像工件。生产 Helm chart MUST NOT 默认渲染 `:latest`，也 MUST NOT 通过 `Chart.appVersion`、空 tag fallback、环境 values 或命令行覆盖接受 `latest` 作为发布镜像。

#### Scenario: 默认 Helm 渲染禁止 latest

- **WHEN** 渲染 user-service Helm chart 的生产基线 values
- **THEN** chart MUST 要求调用方显式提供不可变 image ref
- **AND** 渲染结果 MUST NOT 包含 `image: *:latest`
- **AND** 缺少 image ref 时 Helm lint 或 Helm template MUST 失败

#### Scenario: 显式 latest 覆盖被拒绝

- **WHEN** 发布方通过 values 文件或 `--set` 将 user-service 镜像设置为 `latest` tag
- **THEN** Helm lint 或 Helm template MUST 失败
- **AND** 系统 MUST NOT 生成 Deployment 或 RBAC seed Job manifest

#### Scenario: 同一发布工件贯穿 CI 与 Helm

- **WHEN** CI/CD 构建准备发布的 user-service 镜像
- **THEN** pipeline MUST 推送 `sha-<commit>` tag 或解析 registry digest，并记录该镜像身份
- **AND** 漏洞扫描、SBOM、镜像内容断言和 Helm 发布 MUST 使用同一 image ID、digest 或等价不可变引用
- **AND** Deployment 与 RBAC seed Job MUST 使用完全相同的不可变 image ref

#### Scenario: 回滚使用不可变历史引用

- **WHEN** Helm release 需要回滚 user-service 镜像
- **THEN** 发布方 MUST 回退到上一版已记录的不可变 image ref
- **AND** 回滚流程 MUST NOT 将镜像改回 `latest` 或依赖 registry 当前 tag 指向

