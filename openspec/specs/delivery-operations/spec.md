## Purpose

定义 AegisCore 的交付运维能力，覆盖构建、运行、测试、lint、生成、数据库迁移、容器、部署资产和发布顺序。

## Requirements

### Requirement: 构建、本地运行与 CLI 生命周期

系统 MUST 通过统一 Makefile 和 user-service CLI 提供可重复的构建、运行及进程生命周期控制，并从已加载配置获取启动和停止预算。

#### Scenario: 构建和运行 user-service

- **WHEN** 协作者执行 `make build`
- **THEN** 系统 MUST 将 user-service 二进制构建到 `USER_SERVICE_BIN`
- **WHEN** 执行 `make user-service-run`
- **THEN** 系统 MUST 使用 `USER_SERVICE_CONFIG` 启动 `aegiscore-user-services serve`

#### Scenario: 命令帮助和稳定 surface

- **WHEN** 协作者执行根或 user-service help
- **THEN** 系统 MUST 输出可用命令及中文说明
- **AND** `serve`、`rbac`、`fxgraph` 的名称、公开 flag、默认配置路径、退出码和输出语义 MUST 保持稳定

#### Scenario: lifecycle timeout 配置

- **WHEN** 未声明 `runtime.lifecycle.start_timeout` 和 `stop_timeout`
- **THEN** 系统 MUST 使用共享配置层提供的正数默认值，且 stop 总预算 MUST 不小于 HTTP 和 gRPC shutdown timeout
- **WHEN** 显式值非正数或 stop 总预算小于协议关闭预算
- **THEN** 配置加载 MUST 失败并返回可定位错误
- **AND** CLI MUST NOT 重复定义默认 timeout 常量

#### Scenario: 外部与内部退出协调

- **WHEN** 上游 context 取消或 `App.Wait()` 返回 shutdown signal
- **THEN** serve 命令 MUST 使用未被取消的上游 context value 和配置化预算调用且仅调用一次 `App.Stop()`
- **AND** 非零内部 exit code 或 Stop error MUST 转换为保留全部诊断信息的 Cobra error
- **AND** 命令内部 MUST NOT 调用 `os.Exit`

#### Scenario: CLI 测试依赖局部化

- **WHEN** 测试替换 lifecycle app factory 或 RBAC runner
- **THEN** 替身 MUST 通过命令实例或函数调用范围内的依赖注入提供
- **AND** 正式代码 MUST NOT 暴露 package-level 可变函数变量或 test-only hook

### Requirement: 测试、lint 与完整验证门禁

系统 MUST 提供模块级和仓库级测试、lint、架构检查与完整 verify 入口；OpenSpec change 必须在全部实现和文档完成、预期变更暂存后执行最终门禁。

#### Scenario: 统一质量命令

- **WHEN** 执行 `make test`
- **THEN** 系统 MUST 运行 `common` 和 `user-service` 测试
- **WHEN** 执行 `make lint`
- **THEN** 系统 MUST 运行各 Go module 的 `golangci-lint`
- **WHEN** 执行 `make verify`
- **THEN** 系统 MUST 覆盖 lint、架构检查、测试、必要生成并通过 `git diff --exit-code` 检测 drift

#### Scenario: OpenSpec 最终验证顺序

- **WHEN** change 准备完成
- **THEN** 实现、规格和文档任务 MUST 已完成，本次预期变更 MUST 已暂存
- **AND** 相关单元测试、`make lint` 和 `make verify` MUST 全部通过

#### Scenario: CI 使用有效前缀目标

- **WHEN** GitHub Actions 检查 user-service 架构、OpenAPI 或 migration
- **THEN** MUST 分别调用 `make user-service-architecture-lint`、`make user-service-openapi-generate` 和 `make user-service-migrate-validate`
- **AND** MUST NOT 调用根 Makefile 中不存在或缺少服务前缀的私有目标

### Requirement: Go 测试与生成物约定

Go 测试 MUST 使用语义化断言和消费侧 mock，mock 与 metrics no-op MUST 通过可复现生成入口维护并纳入 drift 校验，且生成入口不得污染正式构建。

#### Scenario: 语义化断言

- **WHEN** 测试断言错误、对象、数值、集合、字符串、JSON、时间或 panic
- **THEN** MUST 优先使用 `testify/require` 的对应语义化断言
- **AND** 互不依赖的多项检查 MAY 使用 `assert`，本质布尔状态且无更具体断言时 MAY 使用 `True` 或 `False`
- **AND** 测试 MUST NOT 用手写 `if`、机械 `Fail` 或多个基础断言替代已有明确断言

#### Scenario: 生成入口可复现

- **WHEN** package 需要 mock 或 metrics no-op
- **THEN** 对应 module MUST 显式声明生成工具依赖，入口 MUST 归消费 package 所有
- **AND** `mock_generate.go` MUST 使用 `//go:build generate` 排除正常构建，生成物 MUST 位于消费 package 的测试边界
- **AND** auth HTTP use case mock 入口 MUST 位于 auth HTTP transport 本地，不得进入全局 mock 包

#### Scenario: 生成和 drift 检查

- **WHEN** 执行 `make common-generate`、`make user-service-generate` 或完整 verify
- **THEN** 已登记的 mock、metrics no-op 和其他 Go 生成物 MUST 可重建
- **AND** interface、生成指令或生成物不一致时验证 MUST 通过 diff 失败

#### Scenario: 禁止测试专用生产 API

- **WHEN** 人工维护的非测试 Go 文件引入 `ForTest`、`testHook`、可变全局替身或等价测试专用 API
- **THEN** 架构检查 MUST 拒绝该变更
- **AND** 测试 MUST 基于现有实现、局部依赖注入或 `common/testing`，MUST NOT 驱动冗余生产分支和适配层

### Requirement: 架构边界与 Fx 依赖图验证

系统 MUST 通过 `user-service-architecture-lint` 保护 feature-first、分层、共享边界、生成配置和部署契约，并提供无外部副作用的正式 Fx 依赖图诊断入口。

#### Scenario: feature 与共享边界

- **WHEN** 服务业务代码新增到横向 controller/service/repository 或错误归入 `common`、`internal/shared`、`internal/integration`
- **THEN** 架构 lint MUST 失败并要求代码回到所属 feature 或符合既有共享准入规则
- **AND** 当前不存在的 gRPC、MQ、eventbus 或 outbox 模型 MUST NOT 以空壳或推测性实现进入正式边界

#### Scenario: 分层依赖保护

- **WHEN** domain、application 或 infrastructure 新增 import
- **THEN** lint MUST 阻止 domain 导入 Gin、Ent、Redis、config、logger、response、application port 或 adapter
- **AND** application MUST NOT 导入 HTTP transport DTO 或 Ent predicate 包
- **AND** HTTP controller MUST 先使用 `binding.BindOrAbort`，再使用 feature-local input preparer

#### Scenario: 生成 Fx 图

- **WHEN** 执行 `cd user-service && go run ./cmd fxgraph --config ./configs/config.yaml --output /tmp/aegis-fx.dot`
- **THEN** 系统 MUST 基于正式 App module 和正式配置投影生成非空 DOT
- **AND** 图 MUST 展示 auth、user、role、permission、providers、router 及关键 metrics 依赖边
- **AND** 生成过程 MUST 使用无副作用资源替身，MUST NOT 连接真实 PostgreSQL、Redis、OTLP 或启动 listener

### Requirement: Ent 生成与 Atlas migration

Ent schema MUST 是数据库结构来源，Atlas SQL migration MUST 是可审查交付工件。仓库 MUST 支持生成、diff、validate 和 hash 校验，但 MUST NOT 提供自动连接目标数据库执行 `atlas migrate apply` 的入口。

#### Scenario: 生成 Ent 与 migration

- **WHEN** Ent schema 或生成特性变化
- **THEN** 协作者 MUST 执行 `make user-service-generate` 和 `make user-service-migrate-diff name=<migration-name>`，审查 SQL 与 `atlas.sum`
- **AND** Ent 生成物 MUST 支持 RBAC bulk insert 的唯一冲突忽略能力

#### Scenario: 校验和受控执行

- **WHEN** migration 准备发布
- **THEN** `make user-service-migrate-validate` MUST 校验 SQL 和 `atlas.sum`
- **AND** SQL MUST 提交 Git，并通过 DBA 工单或受控发布平台执行
- **AND** 手工调整 SQL 后 MUST 刷新 hash 并重新验证

#### Scenario: 禁止运行时 schema 变更和自动 apply

- **WHEN** user-service、E2E、Makefile、脚本或部署资产准备数据库
- **THEN** MUST 使用已提交 SQL migration，运行时代码 MUST NOT 使用 `client.Schema.Create(ctx)` 表达 schema 变更
- **AND** 仓库 MUST NOT 提供 `migrate-apply`、自动 migration Job 或等价 Atlas apply 入口

#### Scenario: 无数据库真实外键

- **WHEN** 生成或审查 user-service SQL migration
- **THEN** migration MUST NOT 包含 `FOREIGN KEY` 或 `REFERENCES`
- **AND** Ent edge、关联字段和必要唯一索引 MUST 保留

#### Scenario: pg_trgm 与本地 dev image

- **WHEN** migration 使用 `gin_trgm_ops`
- **THEN** 首个 migration MUST 在索引前创建 `pg_trgm` 并提示 DBA 权限
- **AND** Atlas dev Dockerfile、diff 脚本、`atlas.hcl` 与 Compose 的本地 image tag MUST 一致，lint MUST 检测 drift

### Requirement: 仓库级 OpenAPI 转换交付

跨服务 OpenAPI 转换库 MUST 位于 `common/http/openapi`，可执行 CLI MUST 位于 `tools/openapi-convert`，服务脚本 MUST 拥有服务专属参数和输出目录。

#### Scenario: user-service 生成文档

- **WHEN** 执行 `make user-service-openapi-generate`
- **THEN** user-service 脚本 MUST 调用仓库级 CLI，将 Swagger 转换为 OpenAPI 3，并更新 `openapi.go`、`openapi.json` 和 `openapi.yaml`

#### Scenario: 服务参数不下沉到通用工具

- **WHEN** 生成需要 server、探活路径、security scheme 或输出路径
- **THEN** user-service 脚本 MUST 显式传参
- **AND** 通用 CLI MUST NOT 写死 `/api/v1`、健康路由或 `BearerAuth` 等服务语义

#### Scenario: 工具测试与依赖升级

- **WHEN** 转换工具或 Swagger UI 依赖变化
- **THEN** 测试 MUST 验证结构化输出、错误路径和当前 v2 静态资源契约
- **AND** 完整验证 MUST 检测生成物及依赖 drift

### Requirement: user-service 运行时镜像与构建

user-service 镜像 MUST 使用 BuildKit、不可变基础镜像、只读 Go module 解析和分离缓存边界构建，并以最小非 root 运行时交付同一可验证镜像工件。

#### Scenario: 可重复构建

- **WHEN** Docker 构建准备依赖和编译
- **THEN** MUST 在源码前复制 `go.work`、workspace module manifests 及存在的 checksum 文件
- **AND** MUST 使用持久化 module/build cache、`-mod=readonly`、`-trimpath` 和显式 VCS metadata 策略
- **AND** 构建 MUST NOT 修改 `go.work.sum`、`go.mod` 或 `go.sum`

#### Scenario: 基础镜像与 CI 工件

- **WHEN** Dockerfile 声明 builder 或 runtime image
- **THEN** 每个基础镜像 MUST 同时保留可读 tag 和审核后的 digest
- **AND** CI 的内容断言、漏洞扫描和 SBOM MUST 复用同一 image ID 或 digest，不得分别冷构建不同工件

#### Scenario: 最小运行时镜像

- **WHEN** user-service 容器启动
- **THEN** 镜像 MUST 以非 root 用户运行，只包含服务运行必需内容，并提供证书及时区数据
- **AND** MUST NOT 包含 Go toolchain、shell、Atlas、migration SQL 或默认 migration apply 入口

#### Scenario: 原生容器健康检查

- **WHEN** 镜像在本地或 Compose 中运行
- **THEN** OCI healthcheck MUST 使用镜像内专用探针二进制或等价无 shell 机制访问 `/livez`
- **AND** Kubernetes MUST 继续使用原生 `httpGet` probes，MUST NOT 依赖容器 healthcheck 命令

### Requirement: Docker-backed 集成测试 CI 门禁

GitHub Actions 的阻塞式 test job MUST 通过唯一开关 `AEGISCORE_TEST_CONTAINERS=1` 执行真实 PostgreSQL、Redis、store 集成测试和完整 user-service HTTP E2E。

#### Scenario: CI 启用真实依赖测试

- **WHEN** PR 或主线 push 执行阻塞式 test job
- **THEN** job MUST 设置 `AEGISCORE_TEST_CONTAINERS=1` 并运行 `make test`
- **AND** MUST NOT 读取 `TEST_CONTAINERS` 兼容别名或仅以 `AEGISCORE_TEST_E2E` 替代完整门禁

#### Scenario: 前置失败不得静默跳过

- **WHEN** 规范开关已启用
- **THEN** Docker daemon、镜像、容器、migration 或配置前置失败 MUST 使 job 失败
- **AND** PostgreSQL/Redis smoke、role store 与 HTTP E2E MUST 实际执行而非 skip

#### Scenario: E2E 配置与范围

- **WHEN** E2E 启动 Fx runtime
- **THEN** harness MUST 提供当前严格配置、真实 PostgreSQL/Redis 和已提交 migration，并覆盖认证与用户 HTTP flow
- **AND** verify、race 或 coverage job MAY 不重复该真实容器负载

### Requirement: Compose 与生产发布顺序

仓库 MUST 维护 Docker、Compose 和观测资产，并确保 SQL migration 受控执行、RBAC seed 和 HTTP rollout 按固定顺序交付；普通运行时镜像和容器不得自动执行 migration。

#### Scenario: 本地 Compose

- **WHEN** 协作者启动 `deployments/compose`
- **THEN** 系统 MUST 提供 user-service 所需数据库、缓存和观测服务
- **AND** SQL migration MUST 在 seed 前完成，seed MUST 在 app 启动前完成
- **AND** Compose MUST NOT 自动执行 `atlas migrate apply`

#### Scenario: 生产发布顺序

- **WHEN** 发布 user-service
- **THEN** 运维 MUST 先确认已提交 SQL migration 经 DBA 工单或受控平台执行，再运行 RBAC seed，按需创建或分配超级管理员，最后滚动 HTTP 副本
- **AND** 任一前置确认或 seed 失败 MUST 阻止 rollout

#### Scenario: 普通容器不迁移

- **WHEN** 普通运行时容器启动
- **THEN** 容器 MUST 直接执行服务或显式 user-service CLI 命令
- **AND** `RUN_MIGRATIONS=true` MUST NOT 触发 Atlas migration

### Requirement: Kubernetes 生产交付

系统 MUST 提供云厂商无关的 user-service Kubernetes 资产，覆盖 Deployment、Service、配置和 Secret 引用、ServiceAccount、必要 RBAC、PDB、HPA、NetworkPolicy、探针、安全上下文、资源约束和 RBAC seed Job。

#### Scenario: HTTP 工作负载基线

- **WHEN** 渲染或部署原生清单
- **THEN** Pod MUST 使用非 root、只读根文件系统、禁止特权升级、收敛 capabilities，并设置 CPU/内存 requests 与 limits
- **AND** liveness、readiness、startup MUST 分别访问 `/livez`、`/readyz`、`/startupz`
- **AND** Deployment MUST NOT 设置 `RUN_MIGRATIONS=true`

#### Scenario: NetworkPolicy 明确来源和目的地

- **WHEN** 清单允许 ingress 或访问 PostgreSQL、Redis、OTLP
- **THEN** ingress MUST 使用明确 namespace 与 pod selector，egress MUST 使用目标 selector 或精确 `ipBlock`
- **AND** MUST NOT 通过空 namespace selector 或仅按端口向任意目的地放行
- **AND** 准入标签 MUST 由 admission policy 或等价控制限制未授权使用

#### Scenario: RBAC seed 前置作业

- **WHEN** 目标数据库已确认完成本 release SQL migration
- **THEN** seed Job MUST 使用当前发布镜像执行 `rbac seed`，并支持系统角色恢复和绑定同步选项
- **AND** seed Job 与 HTTP Deployment MUST 使用同一 release 工件集合

#### Scenario: 原生清单验证

- **WHEN** Kubernetes 资产变化
- **THEN** README 或 tasks MUST 提供 YAML/schema 或 server-side dry-run 命令
- **AND** 验证 MUST 确认不存在自动 Atlas migration Job

### Requirement: Helm chart 交付

系统 MUST 为 `aegiscore-user-services` 提供与原生清单等价的 Helm chart，values MUST 模板化镜像、配置、外部 Secret、资源、探针、扩缩容、PDB、NetworkPolicy、seed Job 和 rollout，且默认不包含真实 secret 或自动 migration apply。

#### Scenario: chart 结构与渲染

- **WHEN** 协作者查看或渲染 chart
- **THEN** `Chart.yaml`、`values.yaml`、templates、README 和环境覆盖示例 MUST 存在
- **AND** Secret MUST 通过 `existingSecret` 或等价引用，Deployment MUST 渲染健康 probes 与资源约束

#### Scenario: seed 与 migration 边界

- **WHEN** values 启用 seed Job
- **THEN** chart MUST 使用 user-service 发布镜像渲染 `rbac seed`
- **AND** MUST NOT 渲染 Atlas migration Job、复制 migration SQL 或依赖运行时镜像中的 Atlas

#### Scenario: 默认网络安全

- **WHEN** 使用默认 values 渲染 NetworkPolicy
- **THEN** ingress MUST 使用明确来源选择器，PostgreSQL、Redis、OTLP egress MUST 包含明确 `to` 目的地
- **AND** 集群外依赖 MUST 通过精确 `ipBlock` 或等价目的地覆盖，MUST NOT 删除 `to` 恢复任意目的放行

#### Scenario: Helm 验证

- **WHEN** chart 变化
- **THEN** 协作者 MUST 执行 `helm lint` 和 `helm template`，检查 seed、Deployment、NetworkPolicy 与无 migration Job 契约

### Requirement: 运行配置与终止预算一致性

本地配置、fixture、Compose、Docker、Kubernetes、Helm、脚本和文档 MUST 使用当前 runtime config 契约；原生 Kubernetes 与 Helm 的默认终止宽限期 MUST 一致并覆盖 Fx Stop 总预算及平台余量。

#### Scenario: 当前配置路径

- **WHEN** 交付 user-service 配置
- **THEN** server MUST 使用 `server.http` 与默认禁用的 `server.grpc`，资源 MUST 使用 `resources.redis` 和 `resources.postgres`
- **AND** 环境变量 MUST 使用当前嵌套路径，进程时区 MUST 使用 `TZ`，secret MUST 通过环境变量或 Secret 注入
- **AND** 旧顶层 HTTP、Redis、PostgreSQL、`local_cache`、文件日志或旧 tracing exporter 字段 MUST NOT 被描述为有效配置

#### Scenario: 观测与入口配置

- **WHEN** 生成部署配置
- **THEN** 日志 MUST 仅输出 stdout/stderr，tracing 启用时 MUST 使用 OTLP endpoint
- **AND** pprof MUST 默认关闭并通过 loopback 与受控端口转发临时访问，trusted proxy MUST 由入口策略治理

#### Scenario: 默认终止预算

- **WHEN** 默认 `runtime.lifecycle.stop_timeout` 为 120 秒
- **THEN** 原生 Kubernetes 和 Helm 的 `terminationGracePeriodSeconds` MUST 同为 150 秒，满足 120 秒总预算加至少 30 秒平台余量
- **AND** 正常关闭提前完成时 Pod MUST 立即退出

#### Scenario: 结构化一致性检查

- **WHEN** 默认 lifecycle timeout、preStop、原生清单或 Helm values 变化
- **THEN** 结构化测试 MUST 解析目标配置并在字段缺失、值无效、入口漂移或余量不足时失败
- **AND** 测试 MUST NOT 依赖正则文本误匹配注释或无关字段

#### Scenario: 发布期间优雅终止

- **WHEN** 滚动发布、缩容、驱逐或故障退出终止 Pod
- **THEN** kubelet MUST 为完整 Fx Stop 链路与平台阶段保留默认宽限期
- **AND** 只有宽限期耗尽且进程仍未退出时才能强制终止
