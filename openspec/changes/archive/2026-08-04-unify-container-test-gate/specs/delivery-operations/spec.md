## MODIFIED Requirements

### Requirement: 镜像、部署与受控发布

user-service 镜像 MUST 使用 BuildKit、不可变基础镜像、只读 Go module 解析和分离缓存构建，并以最小非 root 运行时交付同一可验证工件。CI MUST 运行覆盖 common 与 user-service 的真实依赖测试。Docker、Compose、Kubernetes、Helm 和观测资产 MUST 使用一致的 runtime config、release 工件、安全上下文、探针、Secret、资源和网络边界，MUST NOT 包含真实 secret 或自动 migration apply。外部服务标识与镜像名 MUST 使用 `aegiscore-user-service`，容器、二进制和本地目录语义 MUST 使用 `user-service`，MUST NOT 保留旧复数命名。

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
- **THEN** job MUST 仅运行根 `make test-containers`，由 common 与 user-service 模块 target 显式传递唯一 `-aegiscore.testcontainers` flag，MUST NOT 调用 service-local 容器入口或再次运行普通 `make test`
- **AND** 门禁 MUST 使用 `-v -count=1` 输出实际执行的测试名与耗时并禁止测试缓存，覆盖 common PostgreSQL/Redis fixture、permission/role PostgreSQL 集成测试和 user-service HTTP E2E
- **WHEN** 该专用入口启用
- **THEN** Docker daemon、镜像、容器、migration 或配置前置失败 MUST 使 job 失败，Docker-backed 测试 MUST 实际执行而非 skip
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
- **AND** 根 `make test-containers` 启用的 Redis Cluster 兼容测试 MUST 覆盖 auth、RBAC、health 和 metrics 的 Cluster-sensitive 行为
