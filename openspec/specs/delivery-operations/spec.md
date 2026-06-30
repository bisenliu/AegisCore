## Purpose

定义 AegisCore 的交付运维能力，覆盖构建、运行、测试、lint、架构检查、代码生成、数据库迁移、部署资产和发布顺序。

## Requirements

### Requirement: 构建与本地运行

系统 MUST 提供统一 Makefile 入口构建和运行 user-service，并支持通过配置文件启动服务。

#### Scenario: 构建全部服务

- **WHEN** 协作者执行 `make build`
- **THEN** 系统 MUST 构建 user-service 二进制到配置的 `USER_SERVICE_BIN`

#### Scenario: 运行 user-service

- **WHEN** 协作者执行 `make user-service-run`
- **THEN** 系统 MUST 使用 `USER_SERVICE_CONFIG` 指定的 YAML 配置运行 `aegiscore-user-services serve`

#### Scenario: 查看命令帮助

- **WHEN** 协作者执行 `make help` 或 `make -C user-service help`
- **THEN** 系统 MUST 输出可用命令及中文说明

### Requirement: 测试、lint 和完整验证

系统 MUST 提供统一测试、lint、架构边界检查和完整 verify 入口。

#### Scenario: 运行全部测试

- **WHEN** 协作者执行 `make test`
- **THEN** 系统 MUST 运行 `common` 和 `user-service` 的 Go 测试

#### Scenario: 运行 lint

- **WHEN** 协作者执行 `make lint`
- **THEN** 系统 MUST 运行各 Go 模块的 `golangci-lint`

#### Scenario: 运行完整验证

- **WHEN** 协作者执行 `make verify`
- **THEN** 系统 MUST 依次执行 lint、user-service 架构边界检查、测试、OpenAPI 生成，并通过 `git diff --exit-code` 暴露生成物 drift

### Requirement: CI 工作流使用存在的服务前缀根目标

系统 MUST 确保 GitHub Actions 中的交付校验步骤调用仓库根 `Makefile` 中存在的目标；当步骤执行 user-service 私有交付能力时，目标名称 MUST 使用 `user-service-` 前缀。

#### Scenario: PR 门禁运行 user-service 架构 lint

- **WHEN** GitHub Actions PR 或 push verify job 需要执行 user-service 架构边界检查
- **THEN** 工作流 MUST 调用 `make user-service-architecture-lint`

#### Scenario: PR 门禁运行 user-service OpenAPI 生成

- **WHEN** GitHub Actions PR 或 push verify job 需要检查 user-service OpenAPI 生成物是否存在 drift
- **THEN** 工作流 MUST 调用 `make user-service-openapi-generate`
- **THEN** 工作流 MUST 继续通过 `git diff --exit-code` 暴露生成物 drift

#### Scenario: migration 工作流校验 user-service migrations

- **WHEN** GitHub Actions migration validation job 需要校验 user-service Atlas migrations
- **THEN** 工作流 MUST 调用 `make user-service-migrate-validate`

#### Scenario: 禁止无服务上下文目标

- **WHEN** GitHub Actions workflow 需要调用 user-service 私有 lint、生成或 migration 目标
- **THEN** 工作流 MUST NOT 调用根 `Makefile` 中不存在的 `architecture-lint`、`openapi-generate` 或 `migrate-validate` 目标

### Requirement: user-service Fx 依赖图生成

系统 MUST 为 user-service 提供可执行的 Fx 依赖图生成入口，并通过带 `user-service-` 前缀的交付命令暴露给协作者。

#### Scenario: 生成 user-service 依赖图

- **WHEN** 协作者执行 user-service Fx 依赖图生成命令
- **THEN** 系统 MUST 基于 user-service 当前顶层 Fx module 生成依赖图文件
- **AND** 生成过程 MUST 复用 `common/` 中的业务中立 Fx 依赖图 helper

#### Scenario: 根 Makefile 使用服务前缀

- **WHEN** 仓库根 `Makefile` 暴露 user-service Fx 依赖图生成能力
- **THEN** 目标名称 MUST 使用 `user-service-` 前缀
- **AND** 根 `Makefile` MUST NOT 新增无服务上下文的 `fxgraph-generate`、`dependency-graph` 或等价目标

#### Scenario: 依赖图 drift 可检查

- **WHEN** user-service provider、module 或 invoke 关系变化后重新生成依赖图
- **THEN** 系统 MUST 能通过提交的生成物 diff 或专用 check 命令暴露依赖图 drift

### Requirement: 架构边界检查

系统 MUST 提供 user-service 架构 lint，用于检查 feature 分层依赖、禁止跨层违规引用，并校验 OpenSpec/OPSX Markdown 语言约束。

#### Scenario: 分层引用合法

- **WHEN** feature 内代码遵循 domain、application、infrastructure、transport 的依赖方向
- **THEN** `make user-service-architecture-lint` MUST 通过

#### Scenario: 分层引用违规

- **WHEN** 代码出现违反架构边界的 import 或跨 feature 非法依赖
- **THEN** 架构 lint MUST 失败并输出违规位置

#### Scenario: OPSX 文档残留英文模板

- **WHEN** `openspec/specs/`、`openspec/changes/` 或 `docs/opsx/` 下 Markdown 保留默认英文模板标题或说明
- **THEN** 架构 lint MUST 失败并要求改为简体中文正文

#### Scenario: feature-first 组织违规

- **WHEN** 服务内业务代码新增到横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包
- **THEN** 架构 lint 或 review MUST 阻止该变更，并要求代码放入所属 `internal/features/<feature>/`

#### Scenario: HTTP controller 边界

- **WHEN** HTTP controller 处理请求输入
- **THEN** controller MUST 先调用 `binding.BindOrAbort`，再调用一个 feature-local input preparer，且 MUST NOT 直接导入 Ent、Redis client、SQL package 或基础设施 adapter

#### Scenario: domain 依赖保护

- **WHEN** feature domain 层新增 import
- **THEN** domain MUST NOT 导入 Gin、Ent、Redis、config、logger、response envelope、application ports 或 infrastructure adapter

#### Scenario: 生产代码测试专用 API

- **WHEN** 测试需要 fake、stub、fixture、时间控制或特殊断言入口
- **THEN** 这些能力 SHOULD 位于 `_test.go`、`common/testing` 或对应测试基础设施；正式代码 MUST NOT 暴露 `NewXForTest`、`testHook`、`setNowForTest` 等仅为测试服务的 API，除非它们具有清晰运行时职责

#### Scenario: 避免测试驱动的冗余生产代码

- **WHEN** 新增或调整单元测试
- **THEN** 测试 MUST 基于现有实现和合理的可测试性设计；正式代码 MUST NOT 仅为了单元测试而引入与业务无关的额外逻辑、分支、接口或适配层

### Requirement: 代码生成与数据库迁移

系统 MUST 提供 Ent 代码生成、Atlas migration diff、migration validate 和 migration apply 入口，并要求 schema 相关变更同步生成物。

#### Scenario: 生成 Ent 代码

- **WHEN** Ent schema 变化后执行 `make user-service-generate`
- **THEN** 系统 MUST 运行 `go generate ./ent` 并更新 Ent 生成代码

#### Scenario: 生成 migration

- **WHEN** 数据库 schema 变化需要生成 migration
- **THEN** 协作者 MUST 执行 `make user-service-generate` 和 `make user-service-migrate-diff name=<migration-name>` 生成 Ent 代码与 Atlas migration，并审查 SQL 与 `atlas.sum`

#### Scenario: 校验并应用 migration

- **WHEN** migration 准备进入环境或发布流程
- **THEN** 系统 MUST 支持 `make user-service-migrate-validate` 校验 migration，并通过 `DATABASE_URL` 执行 `make user-service-migrate-apply`

#### Scenario: 手动调整 migration SQL

- **WHEN** 生成的 SQL migration 被手动调整
- **THEN** 协作者 MUST 刷新并提交 `atlas.sum`，且 MUST 确保 `make user-service-migrate-validate` 通过

#### Scenario: 运行时不修改 schema

- **WHEN** user-service 正常启动或 E2E 初始化数据库 schema
- **THEN** schema MUST 来自已提交 Atlas SQL migration，运行时服务代码 MUST NOT 使用 `client.Schema.Create(ctx)` 表达 schema 变更

### Requirement: 发布和部署资产

系统 MUST 维护 Docker、Compose、Kubernetes、Helm 和观测部署资产，并明确生产发布中 migration 与 RBAC seed 的顺序。

#### Scenario: 构建 Docker 镜像

- **WHEN** 协作者执行 Docker build 命令并指定 `deployments/docker/user-service.Dockerfile`
- **THEN** 系统 MUST 能从仓库根目录构建 user-service 镜像

#### Scenario: Dockerfile 路径约束

- **WHEN** 调整 user-service Dockerfile、entrypoint 或 COPY 规则
- **THEN** 路径 MUST 继续以仓库根 build context 为基准，容器内迁移脚本和 Atlas 配置 MUST 与 `user-service/migrations/` 当前布局兼容

#### Scenario: 本地 Compose 启动

- **WHEN** 协作者使用 `deployments/compose` 运行本地环境
- **THEN** 系统 MUST 提供 user-service 所需的数据库、缓存和观测服务配置

#### Scenario: Compose 启动顺序

- **WHEN** 使用本地 Compose 启动包含 migration、RBAC seed 和 user-service 的环境
- **THEN** migration MUST 先于 RBAC seed 执行，RBAC seed MUST 先于 user-service app 启动

#### Scenario: 生产发布顺序

- **WHEN** user-service 发布到生产环境
- **THEN** 运维 MUST 先通过独立 migration Job 或 CI/CD release job 执行 user-service `user_db` Atlas migration，再执行 RBAC seed Job，按需显式创建或分配超级管理员，最后启动或滚动更新 HTTP 副本

#### Scenario: 容器启动 migration 兼容模式

- **WHEN** `RUN_MIGRATIONS` 未设置或不为 `true`
- **THEN** user-service 容器 MUST 直接启动服务，不得应用 migration

#### Scenario: 显式启用入口脚本 migration

- **WHEN** `RUN_MIGRATIONS=true`
- **THEN** entrypoint MAY 在启动 HTTP server 前执行 migration，但该模式只适用于简单部署或兼容场景，多副本生产发布 SHOULD 使用独立 migration Job

### Requirement: 用户服务 Kubernetes 生产清单

系统 MUST 为 user-service 提供云厂商无关的 Kubernetes 生产清单，并覆盖运行副本、服务发现、配置引用、Secret 引用、安全上下文、资源配额、探针、滚动更新、PDB、HPA 和 NetworkPolicy。

#### Scenario: 渲染生产清单

- **WHEN** 协作者查看或应用 `deployments/k8s/user-services/` 下的清单
- **THEN** 清单 MUST 包含 user-service 的 `Deployment`、`Service`、配置资源、Secret 引用、`ServiceAccount`、必要 RBAC、`PodDisruptionBudget`、`HorizontalPodAutoscaler` 和 `NetworkPolicy`

#### Scenario: HTTP 副本探针

- **WHEN** user-service Pod 由 Kubernetes Deployment 启动
- **THEN** liveness probe MUST 指向 `GET /livez`，readiness probe MUST 指向 `GET /readyz`，startup probe MUST 指向 `GET /startupz`

#### Scenario: 运行副本不执行 migration

- **WHEN** user-service Deployment 启动普通 HTTP 副本
- **THEN** Pod 环境变量 MUST NOT 设置 `RUN_MIGRATIONS=true`，副本 MUST 只启动 HTTP 服务

#### Scenario: Pod 安全和资源边界

- **WHEN** user-service Pod 被调度
- **THEN** Pod 和容器 securityContext MUST 默认使用非 root、只读根文件系统、禁止特权升级和收敛 Linux capabilities，并且容器 MUST 设置 CPU 与内存 requests/limits

### Requirement: Kubernetes 发布前置作业

系统 MUST 为 user-service Kubernetes 发布提供独立 migration Job 和 RBAC seed Job，并明确它们先于 HTTP Deployment rollout 执行。

#### Scenario: Migration Job

- **WHEN** 发布流水线执行 user-service 数据库迁移
- **THEN** migration Job MUST 使用当前发布镜像执行 `/app/user-service/scripts/migrate-apply.sh`，并 MUST 通过 Secret 或部署系统注入 `DATABASE_URL`

#### Scenario: RBAC seed Job

- **WHEN** migration Job 成功完成后
- **THEN** RBAC seed Job MUST 使用当前发布镜像执行 `rbac seed`，并 MUST 支持 `--reactivate-system` 与 `--sync-system-bindings` 选项

#### Scenario: 发布顺序

- **WHEN** user-service 发布到 Kubernetes 环境
- **THEN** 发布流程 MUST 等待 migration Job 成功，再等待 RBAC seed Job 成功，最后创建或滚动更新 HTTP Deployment

#### Scenario: 前置作业失败

- **WHEN** migration Job 或 RBAC seed Job 失败
- **THEN** 发布流程 MUST 停止 HTTP Deployment rollout，并保留 Job 日志用于诊断

### Requirement: 用户服务 Helm chart

系统 MUST 为 `aegiscore-user-services` 提供 Helm chart，用 values 模板化同等 Kubernetes 交付能力，并保持默认值不包含真实生产 Secret。

#### Scenario: Helm chart 元数据和 values

- **WHEN** 协作者查看 `deployments/helm/aegiscore-user-services/`
- **THEN** chart MUST 包含 `Chart.yaml`、`values.yaml`、templates、README 和环境覆盖示例，并 MUST 暴露 image、service、config、Secret 引用、resources、probes、autoscaling、PDB、NetworkPolicy、migration Job、RBAC seed Job 和 rollout 配置

#### Scenario: Helm 渲染 Secret 引用

- **WHEN** 协作者执行 `helm template` 渲染 chart
- **THEN** 模板 MUST 通过 `existingSecret` 或等价 values 引用外部 Secret，不得默认渲染真实敏感值

#### Scenario: Helm 渲染发布作业

- **WHEN** Helm values 启用 migration Job 和 RBAC seed Job
- **THEN** chart MUST 渲染使用当前发布镜像的两个独立 Job，并保持 Job command 与 Kubernetes 原生清单一致

#### Scenario: Helm Deployment 默认行为

- **WHEN** Helm chart 渲染 user-service Deployment
- **THEN** Deployment MUST 默认不设置 `RUN_MIGRATIONS=true`，并 MUST 渲染 `/livez`、`/readyz`、`/startupz` 探针和资源 requests/limits

### Requirement: Kubernetes 和 Helm 验证说明

系统 MUST 为 user-service Kubernetes 与 Helm 资产提供可执行的验证说明，覆盖模板渲染、YAML/schema 检查和发布顺序检查。

#### Scenario: 验证原生清单

- **WHEN** 协作者修改 `deployments/k8s/user-services/`
- **THEN** tasks 或 README MUST 指明用于校验 YAML、Kubernetes schema 或 server-side dry-run 的命令

#### Scenario: 验证 Helm chart

- **WHEN** 协作者修改 `deployments/helm/aegiscore-user-services/`
- **THEN** tasks 或 README MUST 指明 `helm lint` 和 `helm template` 的验证命令，并说明如何检查 migration Job、RBAC seed Job 和 Deployment 的关键字段

#### Scenario: 架构文档验证

- **WHEN** 部署规格或 OPSX artifacts 变更完成
- **THEN** 协作者 MUST 执行 `make user-service-architecture-lint`，确保中文文档和 OpenSpec 结构约束通过
