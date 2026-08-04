## MODIFIED Requirements

### Requirement: 质量门禁、架构诊断与可复现生成

系统 MUST 提供模块级和仓库级测试、lint、架构检查、生成和完整 verify 入口。测试与生成 MUST 可诊断、可复现且不得扩张正式生产 API。`user-service-architecture-lint` MUST 保护正式架构来源声明的边界；Fx 依赖图诊断 MUST 无外部及运行时激活副作用。OpenAPI 转换库、CLI 和服务脚本 MUST 分别由 `common/http/openapi`、`tools/openapi-convert` 和 user-service 拥有其通用逻辑、可执行入口与服务参数。GitHub Actions MUST 由主 CI workflow 唯一拥有 PR 与主线 push 的标准质量触发，并通过仅支持 `workflow_call` 的复用 workflow 为同一 commit 各执行一次 lint 和普通单测。

#### Scenario: 统一质量与生成门禁

- **WHEN** 执行 `make test`、`make lint` 或 `make verify`
- **THEN** 系统 MUST 分别运行 common 与 user-service 测试、各 module 的 `golangci-lint`，或覆盖 lint、架构检查、测试、必要生成并以 `git diff --exit-code` 检测 drift
- **WHEN** PR 或主线 push 触发 GitHub Actions 标准质量门禁
- **THEN** 主 CI MUST 仅调用一次复用质量 workflow，且同一 commit MUST 只产生一组稳定命名的 `quality / lint` 与 `quality / unit` 检查
- **AND** 复用质量 workflow MUST 仅接受 `workflow_call`，MUST NOT 同时直接监听与主 CI 重叠的 `pull_request` 或主线 `push`
- **AND** CI 的架构/生成检查和 Docker-backed 测试 MUST 使用独立 job，MUST NOT 再次执行 `make lint` 或普通 `make test`
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

- **WHEN** PR 或主线 push 执行阻塞式 `container-test` job
- **THEN** job MUST 运行 `make -C user-service test-containers`，通过唯一 `-aegiscore.testcontainers` flag 启用专用 PostgreSQL/Redis 与 HTTP E2E 包，MUST NOT 再次运行普通 `make test`
- **WHEN** 该专用入口启用
- **THEN** Docker daemon、镜像、容器、migration 或配置前置失败 MUST 使 job 失败，permission/role PostgreSQL 集成测试和 user-service HTTP E2E MUST 实际执行而非 skip
- **AND** E2E harness MUST 使用当前严格配置、真实 PostgreSQL/Redis 和已提交 migration，覆盖认证与用户 HTTP flow；verify、unit、race 或 coverage job MUST NOT 重复该容器负载

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
