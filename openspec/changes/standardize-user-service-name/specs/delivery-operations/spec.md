## MODIFIED Requirements

### Requirement: 构建、运行与稳定 CLI 入口

系统 MUST 通过统一 Makefile 和 user-service CLI 提供可重复的构建、运行及进程生命周期控制。公开 CLI 命令、flag、退出码和错误传播属于运维契约，变更时 MUST 通过对应 capability 明确迁移。user-service 的唯一公开 CLI 根命令 MUST 为 `aegiscore-user-service`，旧 `aegiscore-user-services` 命令名 MUST NOT 作为别名、隐藏命令或兼容入口保留。

#### Scenario: 构建和运行 user-service

- **WHEN** 协作者执行 `make build`
- **THEN** 系统 MUST 将 user-service 二进制构建到 `USER_SERVICE_BIN`
- **WHEN** 执行 `make user-service-run`
- **THEN** 系统 MUST 使用 `USER_SERVICE_CONFIG` 启动 `aegiscore-user-service serve`

#### Scenario: 命令帮助和稳定 surface

- **WHEN** 协作者执行根或 user-service help
- **THEN** 系统 MUST 输出可用命令及中文说明
- **AND** `serve`、`rbac`、`fxgraph` 的名称、公开 flag、默认配置路径、退出码和输出语义 MUST 保持稳定
- **AND** help、测试和文档 MUST 只展示 `aegiscore-user-service` 作为 user-service 根命令

#### Scenario: 外部与内部退出协调

- **WHEN** 上游 context 取消或 `App.Wait()` 返回 shutdown signal
- **THEN** serve 命令 MUST 使用未被取消的上游 context value 和配置化预算调用且仅调用一次 `App.Stop()`
- **AND** 非零内部 exit code 或 Stop error MUST 转换为保留全部诊断信息的 Cobra error
- **AND** 命令内部 MUST NOT 调用 `os.Exit`

### Requirement: 可复现、安全且可验证的运行时镜像与 CI 集成测试

user-service 镜像 MUST 使用 BuildKit、不可变基础镜像、只读 Go module 解析和分离缓存边界构建，并以最小非 root 运行时交付同一可验证镜像工件。GitHub Actions 的阻塞式 test job MUST 通过唯一开关 `AEGISCORE_TEST_CONTAINERS=1` 执行真实 PostgreSQL、Redis、store 集成测试和完整 user-service HTTP E2E。user-service 默认镜像仓库和 CI 镜像名 MUST 使用 `aegiscore-user-service`，容器内二进制名 MUST 使用 `user-service`，旧 `aegiscore-user-services` 镜像名和 `user-services` 二进制名 MUST NOT 保留。

#### Scenario: 可重复构建

- **WHEN** Docker 构建准备依赖和编译
- **THEN** MUST 在源码前复制 `go.work`、workspace module manifests 及存在的 checksum 文件
- **AND** MUST 使用持久化 module/build cache、`-mod=readonly`、`-trimpath` 和显式 VCS metadata 策略
- **AND** 构建 MUST NOT 修改 `go.work.sum`、`go.mod` 或 `go.sum`
- **AND** 构建输出和 ENTRYPOINT MUST 指向 `/app/user-service/bin/user-service`

#### Scenario: 基础镜像与 CI 工件

- **WHEN** Dockerfile 声明 builder 或 runtime image
- **THEN** 每个基础镜像 MUST 同时保留可读 tag 和审核后的 digest
- **AND** CI 的内容断言、漏洞扫描和 SBOM MUST 复用同一 image ID 或 digest，不得分别冷构建不同工件
- **AND** CI 默认 `USER_SERVICE_IMAGE` MUST 使用 `aegiscore-user-service:<sha>`

#### Scenario: 最小运行时镜像

- **WHEN** user-service 容器启动
- **THEN** 镜像 MUST 以非 root 用户运行，只包含服务运行必需内容，并提供证书及时区数据
- **AND** MUST NOT 包含 Go toolchain、shell、Atlas、migration SQL 或默认 migration apply 入口
- **WHEN** 镜像在本地或 Compose 中运行
- **THEN** OCI healthcheck MUST 使用镜像内专用探针二进制或等价无 shell 机制访问 `/livez`
- **AND** Kubernetes MUST 继续使用原生 `httpGet` probes，MUST NOT 依赖容器 healthcheck 命令

#### Scenario: CI 启用真实依赖测试

- **WHEN** PR 或主线 push 执行阻塞式 test job
- **THEN** job MUST 设置 `AEGISCORE_TEST_CONTAINERS=1` 并运行 `make test`
- **AND** MUST NOT 读取 `TEST_CONTAINERS` 兼容别名或仅以 `AEGISCORE_TEST_E2E` 替代完整门禁
- **WHEN** 规范开关已启用
- **THEN** Docker daemon、镜像、容器、migration 或配置前置失败 MUST 使 job 失败
- **AND** PostgreSQL/Redis smoke、role store 与 HTTP E2E MUST 实际执行而非 skip

#### Scenario: E2E 配置与范围

- **WHEN** E2E 启动 Fx runtime
- **THEN** harness MUST 提供当前严格配置、真实 PostgreSQL/Redis 和已提交 migration，并覆盖认证与用户 HTTP flow
- **AND** verify、race 或 coverage job MAY 不重复该真实容器负载

### Requirement: 部署资产一致性与安全基线

仓库 MUST 维护 Docker、Compose、原生 Kubernetes、Helm 和观测资产，使不同交付载体使用一致的当前 runtime config、release 工件、安全上下文、探针、Secret 引用、资源约束和网络边界，且默认不包含真实 secret 或自动 migration apply。user-service 的 Kubernetes、Helm、Compose 和 Docker 默认运行时命名 MUST 统一使用 `aegiscore-user-service` 作为外部服务标识，使用 `user-service` 作为容器名、二进制名或本地服务目录语义，旧复数资源名 MUST NOT 保留为兼容资源。

#### Scenario: Compose 与当前配置

- **WHEN** 协作者启动 `deployments/compose`
- **THEN** 系统 MUST 提供 user-service 所需数据库、缓存和观测服务
- **AND** Compose MUST 使用当前 runtime config 契约和 release 工件
- **AND** SQL migration MUST 在 seed 前完成，seed MUST 在 app 启动前完成
- **AND** Compose MUST NOT 自动执行 `atlas migrate apply`
- **AND** Compose 镜像、healthcheck、Prometheus scrape label 和辅助脚本 MUST 使用 `aegiscore-user-service` 或 `/app/user-service/bin/user-service`

#### Scenario: Kubernetes 和 Helm 工作负载基线

- **WHEN** 渲染或部署原生清单或 Helm chart
- **THEN** Pod MUST 使用非 root、只读根文件系统、禁止特权升级、收敛 capabilities，并设置 CPU/内存 requests 与 limits
- **AND** liveness、readiness、startup MUST 分别访问 `/livez`、`/readyz`、`/startupz`
- **AND** Secret MUST 通过外部引用注入，默认配置和 values MUST NOT 包含真实 secret
- **AND** Deployment 和 chart MUST NOT 设置 `RUN_MIGRATIONS=true` 或渲染 Atlas migration Job、复制 migration SQL、依赖运行时镜像中的 Atlas
- **AND** Deployment、Service、ServiceAccount、ConfigMap、Secret、Job、PDB、HPA、NetworkPolicy、Helm chart 和 release 示例 MUST 使用 `aegiscore-user-service`

#### Scenario: NetworkPolicy 明确来源和目的地

- **WHEN** 清单或 chart 允许 ingress 或访问 PostgreSQL、Redis、OTLP
- **THEN** ingress MUST 使用明确 namespace 与 pod selector，egress MUST 使用目标 selector 或精确 `ipBlock`
- **AND** MUST NOT 通过空 namespace selector 或仅按端口向任意目的地放行
- **AND** 准入标签 MUST 由 admission policy 或等价控制限制未授权使用

#### Scenario: 原生清单与 Helm 等价验证

- **WHEN** Kubernetes 资产或 chart 变化
- **THEN** README 或 tasks MUST 提供 YAML/schema、server-side dry-run、`helm lint` 或 `helm template` 等验证命令
- **AND** 验证 MUST 确认安全上下文、探针、资源、seed、NetworkPolicy 和无 migration Job 契约未漂移
- **AND** 验证 MUST 检查旧 `aegiscore-user-services` 和 `user-services` 运行时命名未残留在当前部署资产中
