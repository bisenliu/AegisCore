## Purpose

定义 AegisCore 的交付运维能力，覆盖构建、运行、测试、lint、生成、数据库迁移、容器、部署资产和发布顺序。
## Requirements
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
- **AND** RBAC help MUST 只展示 `seed` 和 `bootstrap-super-admin` 作为公开 RBAC 运维命令
- **AND** help、测试和文档 MUST 只展示 `aegiscore-user-service` 作为 user-service 根命令
- **AND** help、测试、文档和 Makefile MUST NOT 展示 `rbac create-super-admin`、`rbac assign-super-admin`、`--reset-password`、`ADMIN_RESET_PASSWORD` 或旧超级管理员命令别名

#### Scenario: bootstrap-super-admin Makefile 入口

- **WHEN** 运维执行 `ADMIN_BOOTSTRAP_PASSWORD='<temporary-password>' ADMIN_USERNAME='initial-admin' ADMIN_NICKNAME='Initial Administrator' make user-service-bootstrap-super-admin`
- **THEN** Makefile MUST 调用 `aegiscore-user-service rbac bootstrap-super-admin --username "$ADMIN_USERNAME" --nickname "$ADMIN_NICKNAME" --password-env ADMIN_BOOTSTRAP_PASSWORD`
- **AND** 根 Makefile MUST 提供带服务名前缀的 `user-service-bootstrap-super-admin` 目标
- **AND** 根 Makefile MUST NOT 提供无服务名前缀的 `bootstrap-super-admin` 便利目标
- **AND** Makefile MUST NOT 在命令行中展开、打印或记录密码值
- **AND** 系统 MUST 删除 `user-service-create-super-admin`、`create-super-admin` 和 `ADMIN_RESET_PASSWORD` 入口

#### Scenario: 外部与内部退出协调

- **WHEN** 上游 context 取消或 `App.Wait()` 返回 shutdown signal
- **THEN** serve 命令 MUST 使用未被取消的上游 context value 和配置化预算调用且仅调用一次 `App.Stop()`
- **AND** 非零内部 exit code 或 Stop error MUST 转换为保留全部诊断信息的 Cobra error
- **AND** 命令内部 MUST NOT 调用 `os.Exit`

### Requirement: 仓库质量门禁与可复现生成

系统 MUST 提供模块级和仓库级测试、lint、架构检查、生成和完整 verify 入口。测试与生成 MUST 可诊断、可复现，并不得为了测试便利扩张正式生产 API。

#### Scenario: 统一质量命令

- **WHEN** 执行 `make test`
- **THEN** 系统 MUST 运行 `common` 和 `user-service` 测试
- **WHEN** 执行 `make lint`
- **THEN** 系统 MUST 运行各 Go module 的 `golangci-lint`
- **WHEN** 执行 `make verify`
- **THEN** 系统 MUST 覆盖 lint、架构检查、测试、必要生成并通过 `git diff --exit-code` 检测 drift

#### Scenario: CI 使用有效前缀目标

- **WHEN** GitHub Actions 检查 user-service 架构、OpenAPI 或 migration
- **THEN** MUST 分别调用 `make user-service-architecture-lint`、`make user-service-openapi-generate` 和 `make user-service-migrate-validate`
- **AND** MUST NOT 调用根 Makefile 中不存在或缺少服务前缀的私有目标

#### Scenario: 生成入口可复现

- **WHEN** package 需要 mock、metrics no-op 或其他 Go 生成物
- **THEN** 对应 module MUST 显式声明生成工具依赖，入口 MUST 归消费 package 所有
- **AND** 生成入口 MUST 排除正常构建，生成物 MUST 位于消费 package 的测试边界或所属 feature 边界
- **AND** 执行 `make common-generate`、`make user-service-generate` 或完整 verify 时，已登记生成物 MUST 可重建并通过 diff 暴露 drift

#### Scenario: 禁止测试专用生产 API

- **WHEN** 人工维护的非测试 Go 文件引入仅测试消费、暴露内部状态、全局可变替身或等价测试专用 API
- **THEN** 架构检查 MUST 拒绝该变更
- **AND** 测试 MUST 基于现有实现、局部依赖注入、消费侧最小接口或 `common/testing`，MUST NOT 驱动冗余生产分支和适配层

#### Scenario: 测试可诊断性和硬超时

- **WHEN** 测试构造 Fx app、feature module、启动失败路径或 lifecycle rollback
- **THEN** 测试 MUST 保留可定位 Fx event 或组件日志诊断信息
- **WHEN** 测试直接调用 `Stop(ctx)`、`Shutdown(ctx)`、关闭 hook、worker drain 或其他可能阻塞的函数
- **THEN** 测试 MUST 使用带 timeout 的 context 和测试级等待上限
- **AND** 被测实现不尊重 context 时测试 MUST 在测试级 guard 内失败，而不是等待全局 `go test -timeout`

### Requirement: 架构门禁与无副作用依赖图诊断

系统 MUST 通过 `user-service-architecture-lint` 保护正式架构来源声明的边界，并提供无外部副作用、无运行时激活副作用的正式 Fx 依赖图诊断入口。

#### Scenario: 架构 lint 保护正式边界

- **WHEN** 服务业务代码违反已声明的 feature-first、分层、共享边界、生成配置或部署契约
- **THEN** `user-service-architecture-lint` MUST 失败并指向对应正式架构来源
- **AND** 当前不存在的 gRPC、MQ、eventbus 或 outbox 模型 MUST NOT 以空壳或推测性实现进入正式边界

#### Scenario: 生成 Fx 图

- **WHEN** 执行 `cd user-service && go run ./cmd fxgraph --config ./configs/config.yaml --output /tmp/aegis-fx.dot`
- **THEN** 系统 MUST 基于正式配置投影和无运行时激活的 wiring graph 或专用 graph root 生成非空 DOT
- **AND** 生成过程 MUST NOT 执行生产 runtime `fx.Invoke`，MUST NOT 连接真实 PostgreSQL、Redis、OTLP 或启动 listener
- **AND** 生成过程 MUST NOT 创建 workerpool、本地缓存、tracing exporter 后台资源，MUST NOT 注册真实 route 或 runtime metrics，MUST NOT 修改 `TZ`、`time.Local` 或 Gin mode

#### Scenario: 正式 App 保持完整运行时激活

- **WHEN** user-service 通过 `serve` 命令构建正式 Fx App
- **THEN** 系统 MUST 使用同时包含 wiring module 和 runtime module 的正式 App module
- **AND** HTTP server、pprof server、route 注册、runtime dependency metrics、timezone 初始化、RBAC lifecycle 和 lifecycle hooks MUST 保持正式运行时语义
- **AND** graph 命令的无副作用 root MUST NOT 取代正式 `serve` 的 runtime 激活链路

### Requirement: Ent/Atlas migration 交付

Ent schema MUST 是数据库结构来源，Atlas SQL migration MUST 是可审查交付工件。仓库 MUST 支持生成、diff、validate 和 hash 校验，但 MUST NOT 提供自动连接目标数据库执行 `atlas migrate apply` 的入口。

#### Scenario: Schema、生成物与 migration 可重建

- **WHEN** Ent schema 或生成特性变化
- **THEN** 协作者 MUST 执行 `make user-service-generate` 和 `make user-service-migrate-diff name=<migration-name>`，审查 SQL 与 `atlas.sum`
- **AND** interface、生成指令、Ent 生成物、SQL migration 或 hash 不一致时验证 MUST 通过 diff 或 validate 失败

#### Scenario: migration 校验和受控执行

- **WHEN** migration 准备发布
- **THEN** `make user-service-migrate-validate` MUST 校验 SQL 和 `atlas.sum`
- **AND** SQL MUST 提交 Git，并通过 DBA 工单或受控发布平台执行
- **AND** 手工调整 SQL 后 MUST 刷新 hash 并重新验证
- **AND** user-service、E2E、Makefile、脚本或部署资产准备数据库时 MUST 使用已提交 SQL migration，运行时代码 MUST NOT 使用 `client.Schema.Create(ctx)` 表达 schema 变更
- **AND** 仓库 MUST NOT 提供 `migrate-apply`、自动 migration Job 或等价 Atlas apply 入口

#### Scenario: 数据库结构约束

- **WHEN** 生成或审查 user-service SQL migration
- **THEN** migration MUST NOT 包含 `FOREIGN KEY` 或 `REFERENCES`
- **AND** Ent edge、关联字段和必要唯一索引 MUST 保留
- **WHEN** migration 使用 `gin_trgm_ops`
- **THEN** 首个 migration MUST 在索引前创建 `pg_trgm` 并提示 DBA 权限
- **AND** Atlas dev Dockerfile、diff 脚本、`atlas.hcl` 与 Compose 的本地 image tag MUST 一致，lint MUST 检测 drift

### Requirement: OpenAPI 生成与转换交付

跨服务 OpenAPI 转换库 MUST 位于 `common/http/openapi`，可执行 CLI MUST 位于 `tools/openapi-convert`，服务脚本 MUST 拥有服务专属参数和输出目录。

#### Scenario: user-service 生成文档

- **WHEN** 执行 `make user-service-openapi-generate`
- **THEN** user-service 脚本 MUST 调用仓库级 CLI，将 Swagger 转换为 OpenAPI 3，并更新 `openapi.go`、`openapi.json` 和 `openapi.yaml`

#### Scenario: 服务参数不下沉到通用工具

- **WHEN** 生成需要 server、探活路径、security scheme 或输出路径
- **THEN** user-service 脚本 MUST 显式传参
- **AND** 通用 CLI MUST NOT 写死 `/api/v1`、健康路由或 `BearerAuth` 等服务语义

#### Scenario: 工具测试与 drift

- **WHEN** 转换工具或相关依赖变化
- **THEN** 测试 MUST 验证结构化输出和错误路径
- **AND** 完整验证 MUST 检测生成物及依赖 drift

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

### Requirement: 受控发布顺序与优雅终止

生产发布 MUST 先确认 SQL migration 受控完成，再执行 RBAC seed 和一次性超级管理员 bootstrap，最后滚动 HTTP 副本。Kubernetes 与 Helm 的终止宽限期 MUST 一致，并覆盖 Fx Stop 总预算及平台安全余量。

#### Scenario: 生产发布顺序

- **WHEN** 发布全新数据库上的 user-service
- **THEN** 运维 MUST 先确认已提交 SQL migration 经 DBA 工单或受控平台执行，再运行 RBAC seed，随后运行 `rbac bootstrap-super-admin`，最后启动或滚动 HTTP 副本
- **AND** 初始管理员 MUST 在 HTTP 副本启动后通过强制改密流程把临时密码改为正式密码
- **AND** 任一前置确认、seed 或 bootstrap 失败 MUST 阻止 rollout
- **AND** seed Job 和 bootstrap Job MUST 使用当前发布镜像执行对应 RBAC CLI，并与 HTTP Deployment 使用同一 release 工件集合

#### Scenario: 不支持旧库升级和旧入口兼容

- **WHEN** 发布包含 `rbac bootstrap-super-admin` 的版本
- **THEN** 系统 MUST 只支持全新数据库路径
- **AND** 系统 MUST NOT 支持旧数据库原地升级、旧超级管理员数据识别、bootstrap marker 回填、旧命令别名、双版本 CLI 共存或自动恢复已有管理员
- **AND** 本次变更 MUST NOT 新增 Ent schema、业务表或 Atlas migration

#### Scenario: 普通容器不迁移

- **WHEN** 普通运行时容器启动
- **THEN** 容器 MUST 直接执行服务或显式 user-service CLI 命令
- **AND** `RUN_MIGRATIONS=true` MUST NOT 触发 Atlas migration

#### Scenario: 当前配置与终止预算关系

- **WHEN** 交付 user-service 配置
- **THEN** server MUST 使用 `server.http` 与默认禁用的 `server.grpc`，资源 MUST 使用 `resources.redis` 和 `resources.postgres`
- **AND** user-service 主 PostgreSQL 连接配置路径 MUST 使用 `resources.postgres.primary_db`
- **AND** 环境变量 MUST 使用当前嵌套路径，进程时区 MUST 使用 `TZ`，secret MUST 通过环境变量或 Secret 注入
- **WHEN** 默认 lifecycle timeout、preStop、原生清单或 Helm values 变化
- **THEN** 结构化测试 MUST 解析目标配置并验证 termination grace 不小于 `runtime.lifecycle.stop_timeout` 加平台安全余量，且原生 Kubernetes 与 Helm 默认值一致
- **AND** 测试 MUST NOT 依赖正则文本误匹配注释或无关字段

#### Scenario: 发布期间优雅终止

- **WHEN** 滚动发布、缩容、驱逐或故障退出终止 Pod
- **THEN** kubelet MUST 为完整 Fx Stop 链路与平台阶段保留默认宽限期
- **AND** 正常关闭提前完成时 Pod MUST 立即退出
- **AND** 只有宽限期耗尽且进程仍未退出时才能强制终止

### Requirement: 项目身份初始化与重命名 ID 边界

系统 MUST 区分从基础框架初始化新项目和已有项目重命名。新项目初始化 MAY 生成新的 RBAC 系统 ID namespace 和固化常量；已有项目重命名 MUST NOT 默认重算系统内置 RBAC、permission 或 bootstrap 用户 ID。

#### Scenario: 新项目初始化生成系统 ID
- **WHEN** AegisCore 作为基础框架复制为全新项目
- **THEN** 初始化流程 MAY 生成新的 `SystemIDNamespace`
- **AND** 初始化流程 MAY 基于固定 semantic name 列表生成 UUID v5 结果并写入 `user-service/internal/shared/rbacbaseline/ids.go`
- **AND** 初始化流程 MUST 只修改代码或文档工件，MUST NOT 连接数据库、修改已有数据库数据或执行 RBAC seed

#### Scenario: 已有项目重命名保持系统 ID
- **WHEN** 已落地项目修改项目展示名、服务名、module path、CLI 名、镜像名、部署资源名或观测 label
- **THEN** 重命名流程 MUST NOT 默认修改 `SystemIDNamespace`
- **AND** 重命名流程 MUST NOT 默认修改 `SuperAdminRoleID`、`BootstrapSuperAdminUserID` 或任一 baseline permission ID
- **AND** 重命名流程 MUST NOT 默认修改已有数据库中的角色、权限、用户或绑定 ID
- **AND** 如需重算系统 ID，MUST 作为单独高风险数据迁移 change 处理，不得混入普通重命名流程

#### Scenario: 重命名脚本文档约束
- **WHEN** 仓库提供项目初始化脚本或项目重命名脚本
- **THEN** 文档 MUST 明确初始化脚本只适用于新项目创建
- **AND** 文档 MUST 明确重命名脚本默认不重算系统 ID
- **AND** 脚本或 README MUST NOT 宣称已有项目改名会自动迁移 RBAC 系统 ID

