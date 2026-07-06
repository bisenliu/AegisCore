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

系统 MUST 提供统一测试、lint、架构边界检查和完整 verify 入口。OpenSpec change 的最终 `make lint` 和 `make verify` MUST 在全部实现、规格和文档任务完成后执行，且执行前 MUST 先暂存本次预期变更。

#### Scenario: 运行全部测试

- **WHEN** 协作者执行 `make test`
- **THEN** 系统 MUST 运行 `common` 和 `user-service` 的 Go 测试

#### Scenario: 运行 lint

- **WHEN** 协作者执行 `make lint`
- **THEN** 系统 MUST 运行各 Go 模块的 `golangci-lint`

#### Scenario: OpenSpec 最终 lint 顺序

- **WHEN** 协作者准备完成 OpenSpec change 并执行最终 `make lint`
- **THEN** 协作者 MUST 已完成该 change 的实现、规格和文档任务
- **AND** 协作者 MUST 先将本次预期代码和文档变更加到暂存区

#### Scenario: 运行完整验证

- **WHEN** 协作者执行 `make verify`
- **THEN** 系统 MUST 依次执行 lint、user-service 架构边界检查、测试、OpenAPI 生成，并通过 `git diff --exit-code` 暴露生成物 drift

#### Scenario: OpenSpec 最终 verify 顺序

- **WHEN** 协作者准备完成 OpenSpec change 并执行最终 `make verify`
- **THEN** 协作者 MUST 已完成该 change 的实现、规格和文档任务
- **AND** 协作者 MUST 先将本次预期代码和文档变更加到暂存区
- **AND** `make verify` 的最终 `git diff --exit-code` MUST 用于暴露生成物 drift 或未纳入暂存区的意外变更

### Requirement: Go 测试断言与失败处理

Go 测试 MUST 优先使用 `testify/require` 表达错误、对象、数值、集合、字符串、专属类型和前置条件等语义化断言，并通过立即失败机制减少后续空指针、错误状态级联和手写判断样板。当 `testify` 已提供更具体的语义化断言时，测试 MUST 使用对应断言，而不是通过 `True`、`False`、手写 `if` 或组合多个基础断言来表达同一语义。

#### Scenario: 使用语义化 require 断言

- **WHEN** Go 测试断言错误返回值、对象和值、数值范围、集合、字符串或专属类型行为
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.ErrorContains`、`require.Equal`、`require.NotEqual`、`require.Nil`、`require.NotNil`、`require.Greater`、`require.Less`、`require.GreaterOrEqual`、`require.Len`、`require.Empty`、`require.Contains`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp`、`require.WithinDuration`、`require.Panics` 或等价语义化断言
- **AND** 测试 MUST NOT 使用 `require.True`、`require.False`、手写 `if` 或多个基础断言拼凑上述已有语义化断言可以清晰覆盖的检查

#### Scenario: 布尔状态断言例外

- **WHEN** 测试断言对象自身暴露的布尔状态、channel 是否关闭等本质上就是布尔值的结果，且 `testify` 没有更具体的语义化断言
- **THEN** 测试 MAY 使用 `require.True` 或 `require.False`

#### Scenario: 多个独立失败收集

- **WHEN** 单个测试需要在一次执行中收集多个相互独立的断言失败，且后续检查不依赖前置检查成功
- **THEN** 测试 MAY 使用 `testify/assert` 表达这些独立检查
- **AND** 初始化失败、前置条件失败或后续检查依赖当前结果时，测试 MUST 使用 `require` 立即终止当前测试

#### Scenario: 禁止机械 Fail 替换

- **WHEN** 测试迁移或新增失败处理
- **THEN** 测试 MUST NOT 将手写失败判断机械替换成 `require.FailNow`、`require.FailNowf`、`require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`
- **AND** 存在明确语义化断言时，测试 MUST 优先使用对应的 `require` 或 `assert` 方法

#### Scenario: 保留 testing.T 失败方法例外

- **WHEN** 测试仍直接使用 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf`
- **THEN** 该用法 MUST 属于无法通过现有语义化断言清晰表达的自定义测试控制流、特殊诊断输出，或测试辅助工具不适合依赖 `testify` 的场景
- **AND** 保留原因 MUST 在代码上下文中保持清晰

### Requirement: Go 生成物 drift 校验

系统 MUST 将 mock 生成物和 metrics no-op 生成物纳入 Go 生成与交付验证流程。完整验证 MUST 能在生成物过期、缺失或未提交时通过 drift 检查失败。认证 HTTP controller 使用的 use case mock MUST 由 auth HTTP transport 本地 `mock_generate.go` 声明，并由仓库约定生成命令维护。

#### Scenario: 生成 mock 和 metrics no-op

- **WHEN** 协作者执行仓库约定的 Go 生成命令
- **THEN** 系统 MUST 生成 `go.uber.org/mock/mockgen` mock 文件和 metrics no-op 文件
- **AND** 生成命令 MUST 覆盖 `common` 与 `user-service` 中声明的相关 `go:generate` 入口

#### Scenario: 完整验证发现生成物 drift

- **WHEN** mock 或 metrics no-op 源 interface 变化但生成物未同步
- **THEN** `make verify` 或等价完整验证 MUST 通过重新生成和 `git diff --exit-code` 暴露 drift
- **AND** drift 未解决前 change MUST NOT 被视为完成

#### Scenario: 工具依赖可复现

- **WHEN** 新增或更新 mock 生成入口
- **THEN** 对应 Go module MUST 显式声明 `go.uber.org/mock/mockgen` 工具依赖或等价可复现工具入口
- **AND** 生成流程 MUST NOT 依赖开发者机器上的隐式全局 `mockgen` 二进制

#### Scenario: auth HTTP controller mockgen 入口

- **WHEN** 认证 HTTP controller 测试需要模拟 `LoginUseCase`、`RefreshTokenUseCase`、`ChangePasswordUseCase`、`LogoutCurrentSessionUseCase` 或 `LogoutAllSessionsUseCase`
- **THEN** mockgen 入口 MUST 位于 `user-service/internal/features/auth/transport/http/mock_generate.go`
- **AND** 生成 mock MUST 位于 `auth/transport/http` 测试包内
- **AND** 生成物 MUST NOT 进入全局 `mocks/` 目录或跨 feature mock 包

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

系统 MUST 提供 Ent 代码生成、Atlas migration diff、migration validate 和 migration hash 校验入口，并要求 schema 相关变更同步生成物。系统 MUST NOT 提供通过仓库 Makefile、脚本或部署资产直接连接数据库并执行 `atlas migrate apply` 的入口。user-service Ent 生成配置 MUST 启用支持 RBAC bulk insert 唯一冲突忽略所需的生成特性。

#### Scenario: 生成 Ent 代码

- **WHEN** Ent schema 或 Ent 生成特性变化后执行 `make user-service-generate`
- **THEN** 系统 MUST 运行 `go generate ./ent` 并更新 Ent 生成代码
- **AND** 生成代码 MUST 支持 RBAC 批量写入路径使用 bulk create 的唯一冲突忽略能力

#### Scenario: 生成 migration

- **WHEN** 数据库 schema 变化需要生成 migration
- **THEN** 协作者 MUST 执行 `make user-service-generate` 和 `make user-service-migrate-diff name=<migration-name>` 生成 Ent 代码与 Atlas migration，并审查 SQL 与 `atlas.sum`

#### Scenario: 校验 migration

- **WHEN** migration 准备进入环境或发布流程
- **THEN** 系统 MUST 支持 `make user-service-migrate-validate` 校验已提交 SQL migration 和 `atlas.sum`
- **AND** 系统 MUST NOT 支持通过 `DATABASE_URL` 执行 `make user-service-migrate-apply` 或等价仓库命令连接数据库自动应用 migration

#### Scenario: 手动调整 migration SQL

- **WHEN** 生成的 SQL migration 被手动调整
- **THEN** 协作者 MUST 刷新并提交 `atlas.sum`，且 MUST 确保 `make user-service-migrate-validate` 通过

#### Scenario: 受控执行 SQL migration

- **WHEN** SQL migration 已通过 validate 并准备进入目标数据库
- **THEN** 协作者 MUST 将 SQL migration 和权限要求提交到 Git，并通过 DBA 工单或受控发布平台人工或受控执行
- **AND** 仓库文档 MUST 将标准流程描述为 Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或受控发布平台执行

#### Scenario: pg_trgm 扩展前置

- **WHEN** SQL migration 使用 `gin_trgm_ops` 或其他 `pg_trgm` 能力
- **THEN** 首个 SQL migration 文件 MUST 在相关索引创建前包含 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`
- **AND** 文档或任务 MUST 提醒生产库执行该语句可能需要 DBA 权限

#### Scenario: 运行时不修改 schema

- **WHEN** user-service 正常启动或 E2E 初始化数据库 schema
- **THEN** schema MUST 来自已提交 Atlas SQL migration，运行时服务代码 MUST NOT 使用 `client.Schema.Create(ctx)` 表达 schema 变更

#### Scenario: Ent 生成特性 drift 检查

- **WHEN** user-service Ent 生成特性发生变化但生成物未同步
- **THEN** `make verify` 或等价完整验证 MUST 通过重新生成和 `git diff --exit-code` 暴露 drift

### Requirement: 仓库级 OpenAPI 转换工具

系统 MUST 将跨服务复用的 OpenAPI 转换 CLI 维护在仓库级 `tools/openapi-convert/`，并通过服务脚本传入服务专属生成参数。OpenAPI 转换核心 MUST 保持在 `common/http/openapi`，服务 `internal/` 目录 MUST NOT 承载该通用转换 CLI。

#### Scenario: user-service 生成 OpenAPI

- **WHEN** 协作者执行 `make user-service-openapi-generate`
- **THEN** user-service 生成脚本 MUST 调用 `tools/openapi-convert` 完成 Swagger 2 到 OpenAPI 3 的转换
- **AND** 系统 MUST 更新 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`

#### Scenario: 服务专属生成参数

- **WHEN** 服务生成 OpenAPI 文档时需要配置业务 server、root server、探活路径、security scheme 或输出路径
- **THEN** 对应服务脚本 MUST 显式传入这些参数
- **AND** `tools/openapi-convert` MUST NOT 写死 user-service 的 `/api/v1`、`/livez`、`/readyz`、`/startupz` 或 `BearerAuth` 作为服务语义默认值

#### Scenario: 工具归属边界

- **WHEN** 仓库维护 Swagger/OpenAPI 转换能力
- **THEN** 可复用转换库 MUST 位于 `common/http/openapi`
- **AND** 可执行转换 CLI MUST 位于 `tools/openapi-convert`
- **AND** `user-service/internal/tools/openapi-convert` MUST 不存在

### Requirement: 发布和部署资产

系统 MUST 维护 Docker、Compose、Kubernetes、Helm 和观测部署资产，并明确生产发布中数据库 SQL 执行与 RBAC seed 的顺序。user-service 普通运行时镜像 MUST NOT 包含 Atlas 二进制；仓库提供的部署资产 MUST NOT 自动执行 Atlas migration apply，数据库 SQL migration MUST 由 DBA 工单或受控发布平台在 HTTP rollout 前完成。

#### Scenario: 构建 Docker 镜像

- **WHEN** 协作者执行 Docker build 命令并指定 `deployments/docker/user-service.Dockerfile`
- **THEN** 系统 MUST 能从仓库根目录构建 user-service 运行时镜像
- **AND** 运行时镜像 MUST NOT 包含 `/usr/local/bin/atlas` 或其他 Atlas CLI 二进制

#### Scenario: Dockerfile 路径约束

- **WHEN** 调整 user-service Dockerfile、migration 相关 Dockerfile 或 COPY 规则
- **THEN** 路径 MUST 继续以仓库根 build context 为基准
- **AND** user-service 运行时镜像 MUST NOT 为了执行 migration 而复制 `user-service/migrations/` 或 Atlas 二进制
- **AND** 仓库 Docker 资产 MUST NOT 暴露默认执行 `atlas migrate apply` 的入口

#### Scenario: 本地 Compose 启动

- **WHEN** 协作者使用 `deployments/compose` 运行本地环境
- **THEN** 系统 MUST 提供 user-service 所需的数据库、缓存和观测服务配置
- **AND** Compose 资产 MUST NOT 自动执行 `atlas migrate apply`

#### Scenario: Compose 启动顺序

- **WHEN** 使用本地 Compose 启动包含 RBAC seed 和 user-service 的环境
- **THEN** RBAC seed MUST 只在目标数据库已完成对应 SQL migration 后执行，RBAC seed MUST 先于 user-service app 启动

#### Scenario: 生产发布顺序

- **WHEN** user-service 发布到生产环境
- **THEN** 运维 MUST 先确认 user-service `user_db` 的已提交 SQL migration 已通过 DBA 工单或受控发布平台执行，再执行 RBAC seed Job，按需显式创建或分配超级管理员，最后启动或滚动更新 HTTP 副本

#### Scenario: 普通容器启动不执行 migration

- **WHEN** user-service 普通运行时容器启动
- **THEN** 容器 MUST 直接启动服务或执行显式传入的 user-service CLI 命令，不得应用 migration
- **AND** `RUN_MIGRATIONS=true` MUST NOT 使普通运行时镜像尝试执行 Atlas migration

#### Scenario: 禁止显式自动 apply 入口

- **WHEN** 简单部署、Compose、Kubernetes、Helm 或发布文档描述 HTTP 服务启动前的数据库准备
- **THEN** 部署流程 MUST 指向已提交 SQL migration 的 DBA 工单或受控发布平台执行结果
- **AND** 仓库资产 MUST NOT 提供可直接运行 `atlas migrate apply` 的 migration Job、service、Helm 默认 command 或 Makefile 目标

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

系统 MUST 为 user-service Kubernetes 发布提供 RBAC seed Job，并明确数据库 SQL migration 已由 DBA 工单或受控发布平台完成后，才允许执行 RBAC seed 和 HTTP Deployment rollout。仓库 Kubernetes 资产 MUST NOT 提供自动执行 `atlas migrate apply` 的 migration Job。

#### Scenario: 数据库迁移前置确认

- **WHEN** 发布流水线准备发布 user-service 到 Kubernetes 环境
- **THEN** 发布流程 MUST 先确认目标环境已执行本 release 对应的已提交 SQL migration
- **AND** 确认来源 MUST 是 DBA 工单、发布平台记录或等价受控执行记录

#### Scenario: RBAC seed Job

- **WHEN** 数据库 SQL migration 已确认完成后
- **THEN** RBAC seed Job MUST 使用当前发布镜像执行 `rbac seed`，并 MUST 支持 `--reactivate-system` 与 `--sync-system-bindings` 选项

#### Scenario: 发布顺序

- **WHEN** user-service 发布到 Kubernetes 环境
- **THEN** 发布流程 MUST 等待数据库 SQL migration 完成确认，再等待 RBAC seed Job 成功，最后创建或滚动更新 HTTP Deployment

#### Scenario: 前置确认或作业失败

- **WHEN** 数据库 SQL migration 未确认完成或 RBAC seed Job 失败
- **THEN** 发布流程 MUST 停止 HTTP Deployment rollout，并保留可诊断记录

#### Scenario: 镜像版本一致性

- **WHEN** 发布系统设置 user-service 镜像
- **THEN** RBAC seed Job 和 HTTP Deployment MUST 使用同一 release 工件集合或同一 release tag
- **AND** 发布说明 MUST 禁止混用新版 user-service 运行时镜像和旧版 RBAC seed Job 模板

### Requirement: 用户服务 Helm chart

系统 MUST 为 `aegiscore-user-services` 提供 Helm chart，用 values 模板化 Kubernetes 交付能力，并保持默认值不包含真实生产 Secret。chart MUST 支持 RBAC seed Job 和 HTTP Deployment 使用 user-service 镜像，且 MUST NOT 渲染或默认配置自动执行 `atlas migrate apply` 的 migration Job。

#### Scenario: Helm chart 元数据和 values

- **WHEN** 协作者查看 `deployments/helm/aegiscore-user-services/`
- **THEN** chart MUST 包含 `Chart.yaml`、`values.yaml`、templates、README 和环境覆盖示例，并 MUST 暴露 image、service、config、Secret 引用、resources、probes、autoscaling、PDB、NetworkPolicy、RBAC seed Job 和 rollout 配置
- **AND** chart MUST NOT 暴露默认执行 `atlas migrate apply` 的 migration Job 配置

#### Scenario: Helm 渲染 Secret 引用

- **WHEN** 协作者执行 `helm template` 渲染 chart
- **THEN** 模板 MUST 通过 `existingSecret` 或等价 values 引用外部 Secret，不得默认渲染真实敏感值

#### Scenario: Helm 渲染发布作业

- **WHEN** Helm values 启用 RBAC seed Job
- **THEN** chart MUST 渲染 RBAC seed Job
- **AND** RBAC seed Job MUST 使用 user-service 发布镜像执行 `rbac seed`
- **AND** chart MUST NOT 渲染自动执行 `atlas migrate apply` 的 migration Job

#### Scenario: Helm Deployment 默认行为

- **WHEN** Helm chart 渲染 user-service Deployment
- **THEN** Deployment MUST 默认不设置 `RUN_MIGRATIONS=true`，并 MUST 渲染 `/livez`、`/readyz`、`/startupz` 探针和资源 requests/limits
- **AND** Deployment 使用的 user-service 镜像 MUST NOT 依赖 Atlas 二进制或 migration SQL 文件启动 HTTP 服务

### Requirement: Kubernetes 和 Helm 验证说明

系统 MUST 为 user-service Kubernetes 与 Helm 资产提供可执行的验证说明，覆盖模板渲染、YAML/schema 检查和发布顺序检查。

#### Scenario: 验证原生清单

- **WHEN** 协作者修改 `deployments/k8s/user-services/`
- **THEN** tasks 或 README MUST 指明用于校验 YAML、Kubernetes schema 或 server-side dry-run 的命令
- **AND** 验证说明 MUST 包含检查仓库清单不再提供自动执行 `atlas migrate apply` 的 migration Job

#### Scenario: 验证 Helm chart

- **WHEN** 协作者修改 `deployments/helm/aegiscore-user-services/`
- **THEN** tasks 或 README MUST 指明 `helm lint` 和 `helm template` 的验证命令，并说明如何检查 RBAC seed Job 和 Deployment 的关键字段
- **AND** 验证说明 MUST 包含检查 Helm 渲染结果不再包含自动执行 `atlas migrate apply` 的 migration Job

#### Scenario: 架构文档验证

- **WHEN** 部署规格或 OPSX artifacts 变更完成
- **THEN** 协作者 MUST 执行 `make user-service-architecture-lint`，确保中文文档和 OpenSpec 结构约束通过

### Requirement: common mockgen 交付验证

系统 MUST 为 `common` 模块提供可复现的 mockgen 工具入口、生成命令和 drift 校验。仓库完整验证 MUST 覆盖 `common` 中声明的 mock 生成物，生成物过期、缺失或未提交时 MUST 通过 `git diff --exit-code` 或等价 drift 检查失败。

#### Scenario: common 模块声明 mockgen 工具依赖

- **WHEN** `common` 新增或更新 mock 生成入口
- **THEN** `common` Go module MUST 显式声明 `go.uber.org/mock/mockgen` 工具依赖或等价可复现工具入口
- **AND** 生成流程 MUST NOT 依赖开发者机器上的隐式全局 `mockgen` 二进制

#### Scenario: common 生成命令覆盖 go generate

- **WHEN** 协作者执行 `make -C common generate` 或根 `make common-generate`
- **THEN** 系统 MUST 执行 `common` 模块内的 `go generate ./...`
- **AND** 该命令 MUST 覆盖 `common/security/casbin` 和 `common/http/middleware` 中声明的 mockgen 入口

#### Scenario: common verify 暴露生成物 drift

- **WHEN** 协作者执行 `make common-verify` 或 `make -C common verify`
- **THEN** 系统 MUST 运行 common lint、common 生成和 common 测试
- **AND** 系统 MUST 通过 drift 检查暴露 common 生成物缺失、过期或未提交

#### Scenario: 完整 verify 覆盖 common 生成物

- **WHEN** 协作者执行根 `make verify`
- **THEN** 系统 MUST 在完整验证链路中执行 common 生成命令
- **AND** 最终 `git diff --exit-code` MUST 能暴露 common mock 生成物 drift 或未纳入暂存区的意外变更

### Requirement: user-service 运行时装配测试断言迁移

`user-service/internal/bootstrap` 与 `user-service/internal/providers` 中覆盖 Fx provider、bootstrap validation、PostgreSQL/Redis/Ent provider、Gin engine、routes provider 和 HTTP server lifecycle 的测试 MUST 使用 `docs/TESTING.md` 规定的语义化断言。断言迁移 MUST 保持 Fx 依赖图、provider 输出、生命周期 hook、server start/stop、graceful shutdown、forced close、drain tracker 和配置默认值语义不变。

#### Scenario: Fx provider 和 bootstrap validation 断言

- **WHEN** provider 或 bootstrap 测试验证 `fx.ValidateApp`、named resource、provider 输出、配置默认值、lifecycle hook 数量、启动日志或关闭顺序
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.NotNil`、`require.Len`、`require.Equal`、`require.ElementsMatch` 或等价语义化断言
- **AND** 多个互相独立的 provider 输出或日志字段 MAY 使用 `assert`
- **AND** 迁移 MUST NOT 改变 Fx module、provider、invoke、named resource 或 bootstrap validation 生产行为

#### Scenario: HTTP server lifecycle 断言

- **WHEN** bootstrap 测试验证 listener bind 失败、Serve 错误、Shutdown 错误、lifecycle context cancellation、active handler drain、forced close、default shutdown timeout 或 drain tracker wait 行为
- **THEN** 测试 MUST 使用语义化断言表达错误、错误包含关系、调用次数、耗时边界、日志字段和 server timeout 配置
- **AND** channel handoff、blocked handler、goroutine 退出等待或跨 goroutine 错误传递等测试控制流 MAY 保留符合 `docs/TESTING.md` 例外规则的直接 `testing.T` 失败调用
- **AND** 迁移 MUST NOT 改变 HTTP server start/stop、graceful shutdown、forced close、drain tracker 或 Fx lifecycle 语义

#### Scenario: 残留失败调用受扫描约束

- **WHEN** 目标范围 `_test.go` 保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail*` 或 `assert.Fail*`
- **THEN** 每个剩余命中 MUST 属于并发协调、特殊诊断输出、测试辅助工具边界或无法通过现有语义化断言清晰表达的控制流
- **AND** change tasks MUST 列明剩余例外及原因

### Requirement: 运行时装配断言迁移不扩大交付范围

断言迁移 MUST 只覆盖 issue 指定的 router、providers 和 bootstrap 测试路径。系统 MUST NOT 将本 change 扩展为 feature、cmd、Ent schema、e2e、common、部署资产或 OpenAPI 生成物迁移。

#### Scenario: 实施范围受限

- **WHEN** 实施本 change
- **THEN** 代码修改 MUST 限定在 `user-service/internal/router/**/*_test.go`、`user-service/internal/providers/**/*_test.go`、`user-service/internal/bootstrap/**/*_test.go` 和本 change 的 OpenSpec artifacts
- **AND** change MUST NOT 修改生产 Go 文件、Ent schema、Atlas migration、OpenAPI 生成物、部署清单或 `common` 测试

### Requirement: user-service CLI 与 Ent schema 测试断言迁移

`user-service/cmd` 与 `user-service/ent/schema` 中覆盖 CLI command、flag/env normalization、cleanup error、Ent schema field、edge、index、annotation、default、validator 和 mixin 的测试 MUST 使用 `docs/TESTING.md` 规定的语义化断言。断言迁移 MUST 保持服务前缀 Make target、CLI command graph、命令帮助输出约束、Ent schema 定义、Atlas migration 和生成物交付流程不变。

#### Scenario: CLI command 断言

- **WHEN** cmd 测试验证 root command、serve command、RBAC command、flag 绑定、env normalization、command output、usage 文本、cleanup error 或执行错误
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.NotNil`、`require.Len`、`require.Equal`、`require.Contains`、`require.Regexp` 或等价语义化断言
- **AND** 多个互相独立的 command property MAY 使用 `assert`
- **AND** 迁移 MUST NOT 新增旧 root command alias、旧 flag/env 名、旧 usage 文本或无服务前缀 Make target 兼容断言

#### Scenario: Ent schema 断言

- **WHEN** Ent schema 测试验证 field 数量、field 名称、类型、唯一性、可选性、默认值、validator、edge、index、annotation、mixin 或 schema comment
- **THEN** 测试 MUST 使用 `require.Len`、`require.Equal`、`require.NotNil`、`require.Empty`、`require.NotEmpty`、`require.ElementsMatch`、`require.Contains`、`require.Greater`、`require.Regexp` 或等价语义化断言
- **AND** 多个互相独立的 field、edge、index 或 annotation 检查 MAY 使用 `assert`
- **AND** 迁移 MUST NOT 修改 Ent schema、Ent 生成代码、Atlas migration 或 schema 运行时行为

#### Scenario: 残留失败调用受扫描约束

- **WHEN** 目标范围 `_test.go` 保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail*` 或 `assert.Fail*`
- **THEN** 每个剩余命中 MUST 属于特殊测试控制流、特殊诊断输出、测试辅助工具边界或无法通过现有语义化断言清晰表达的控制流
- **AND** change tasks MUST 列明剩余例外及原因

### Requirement: cmd 与 Ent schema 断言迁移不扩大交付范围

断言迁移 MUST 只覆盖 issue 指定的 cmd 与 Ent schema 测试路径。系统 MUST NOT 将本 change 扩展为 router/provider/bootstrap、feature、e2e、common、部署资产、OpenAPI 生成物或数据库结构变更。

#### Scenario: 实施范围受限

- **WHEN** 实施本 change
- **THEN** 代码修改 MUST 限定在 `user-service/cmd/**/*_test.go`、`user-service/ent/schema/**/*_test.go` 和本 change 的 OpenSpec artifacts
- **AND** change MUST NOT 修改生产 Go 文件、Ent schema、Ent 生成代码、Atlas migration、OpenAPI 生成物、部署清单或 `common` 测试

### Requirement: 聚合路由测试断言与覆盖率验收
系统 MUST 为 user-service 聚合路由注册补充符合交付断言规范的 Go 测试，并通过覆盖率验证确保 `RegisterUserServiceHTTPRoutes` 和 `registerV1Routes` 均被执行。

#### Scenario: 语义化断言
- **WHEN** 新增或修改 `user-service/internal/router` 路由注册测试
- **THEN** 测试 MUST 优先使用 `require` 的语义化断言表达错误、集合、字符串、长度和包含关系
- **AND** 存在 `Len`、`Contains`、`ElementsMatch`、`ErrorContains`、`Regexp` 或等价更具体断言时，测试 MUST NOT 使用 `True` 或 `False` 包装布尔表达式
- **AND** 只有多个互相独立的 route 条目需要一次性收集失败且后续检查不依赖前置结果时，测试 MAY 使用 `assert`

#### Scenario: router 覆盖率验收
- **WHEN** 本 change 实施完成
- **THEN** `go test -cover ./user-service/internal/router` MUST 通过
- **AND** `go tool cover -func` MUST 显示 `RegisterUserServiceHTTPRoutes` 和 `registerV1Routes` 均有覆盖
- **AND** `openspec validate cover-user-service-route-registration-no-compat` MUST 通过

### Requirement: CLI 命令测试语义化断言

`delivery-operations` 的 user-service 命令测试 MUST 使用语义化断言表达 CLI 错误、参数缺失、依赖初始化、cleanup 合并和命令属性检查。测试 MUST 优先使用 `require` fail-fast 断言；只有互相独立且不影响后续测试前置条件的命令 property 检查 MAY 使用 `assert`。

#### Scenario: 命令错误使用 fail-fast 断言

- **WHEN** CLI 测试验证参数缺失、配置错误、依赖初始化错误、命令执行错误或 cleanup 错误
- **THEN** 测试 MUST 使用 `require.Error`、`require.ErrorContains`、`require.ErrorIs`、`require.ErrorAs` 或等价 fail-fast 断言
- **AND** 测试 MUST NOT 使用 `require.True`、`assert.True` 或手写 `if` 拼装错误断言替代更具体的错误断言

#### Scenario: 后续检查依赖当前命令结果

- **WHEN** 后续断言需要依赖命令执行成功、初始化成功、返回对象非空或 error 类型匹配
- **THEN** 测试 MUST 使用 `require` 断言建立前置条件
- **AND** 失败后 MUST 停止当前测试，避免继续读取无效结果

#### Scenario: 独立命令属性允许 assert

- **WHEN** 多个命令 flag 默认值、短描述、Use 字符串或互相独立的布尔属性彼此不构成前置依赖
- **THEN** 测试 MAY 使用 `assert` 聚合这些独立属性检查
- **AND** 若存在 `Len`、`Contains`、`ElementsMatch`、`Regexp`、`Greater`、`LessOrEqual` 等更具体断言，测试 MUST 优先使用具体断言

#### Scenario: 禁止机械 failure helper 替换

- **WHEN** 新增或修改 user-service 命令测试
- **THEN** 测试 MUST NOT 使用机械 `Fail`、`Failf`、`FailNow`、`FailNowf` 或旧手写断言兼容 helper 表达常见断言
- **AND** `t.Fatal`、`t.Fatalf`、`t.Error` 和 `t.Errorf` 只允许出现在 `docs/TESTING.md` 明确允许的边界内

### Requirement: user-service E2E 测试断言迁移
系统 MUST 将 `user-service/tests` 下的 E2E HTTP flow、migration validation 和测试 harness Go 测试迁移到 `docs/TESTING.md` 固化的统一断言规范。断言迁移 MUST 保持 E2E 流程、测试数据构造、Testcontainers 前置条件、migration 应用顺序和生产行为不变，并 MUST NOT 引入旧断言兼容 helper、旧 API 响应兼容断言或机械 `Fail*` 替换。

#### Scenario: 语义化断言优先
- **WHEN** E2E 测试断言 HTTP status、错误、响应 envelope、集合长度、无序集合、JSON 响应、时间相关结果、文件读取、SQL 执行或对象字段
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.Equal`、`require.NotEmpty`、`require.Empty`、`require.Len`、`require.Greater`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp`、`require.WithinDuration` 或等价语义化断言
- **AND** 存在更具体语义化断言时，测试 MUST NOT 使用 `True`、`False`、手写 if 或多个基础断言拼凑同一检查

#### Scenario: 完整 HTTP flow 独立字段收集
- **WHEN** 完整 HTTP flow 的单个响应包含多个互相独立的字段检查，且后续检查不依赖这些字段全部成功
- **THEN** 测试 MAY 使用 `testify/assert` 收集独立失败
- **AND** 初始化失败、容器或配置前置条件、JSON 解码、数据库连接、migration 应用和后续流程依赖的结果 MUST 使用 `require` 立即终止当前测试

#### Scenario: migration validation 断言迁移
- **WHEN** migration harness 枚举 SQL migration、读取文件、拆分 SQL statement、定位 user-service 根目录或逐条执行 migration
- **THEN** 测试 MUST 使用语义化断言表达错误、空集合、执行失败和路径定位失败
- **AND** 迁移 MUST NOT 改变 SQL parser 对注释、单引号、双引号、dollar quote、statement 分隔和错误返回的处理语义

#### Scenario: 残留失败调用受扫描约束
- **WHEN** 实施完成后扫描 `user-service/tests/**/*_test.go` 中的 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail*` 或 `assert.Fail*`
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的特殊测试控制流、特殊诊断输出、测试辅助工具边界，或验收正则对 `fmt.Errorf` 的 false positive
- **AND** change tasks MUST 列明每个剩余命中及原因

#### Scenario: E2E 断言迁移验证
- **WHEN** 本 change 实施完成
- **THEN** `rg "github.com/stretchr/testify/(require|assert)" user-service/tests --glob "*_test.go"` MUST 定位到迁移后的实际使用点
- **AND** `go test ./user-service/tests/...` MUST 在具备 E2E 容器前置条件时通过
- **AND** 若容器前置条件不可用，tasks MUST 明确记录 `AEGISCORE_TEST_E2E=1` 或通用容器测试开关、Docker 或兼容容器运行时等可运行前置条件和已完成替代验证
- **AND** `openspec validate standardize-e2e-test-assertions-no-compat` MUST 通过

### Requirement: 仓库级工具测试断言验证
仓库级工具测试 MUST 遵循统一 Go 测试断言规范。OpenAPI 转换、CLI 工具输入输出、文件生成和交付验证相关工具测试 MUST 优先使用 `testify/require` 的语义化断言；存在更具体的 `Len`、`ErrorContains`、`Contains`、`ElementsMatch`、`JSONEq`、`Regexp` 等断言时，测试 MUST NOT 使用 `True` / `False` 或手写 `if` 拼装同等检查。

#### Scenario: 迁移工具测试断言
- **WHEN** `tools/**/*_test.go` 或仓库级工具测试断言错误、文件内容、JSON/YAML 输出、集合长度、字符串匹配或生成物路径
- **THEN** 测试 MUST 使用 `require.NoError`、`require.ErrorContains`、`require.Len`、`require.Contains`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp` 或等价语义化断言表达检查
- **AND** 测试 MUST NOT 使用手写 `t.Fatalf` / `t.Errorf` 或 `require.True` / `require.False` 包装可由专属断言表达的检查

#### Scenario: 工具测试包为空
- **WHEN** 当前仓库级 `tools` 范围没有 Go 测试包或没有 `_test.go` 文件
- **THEN** change tasks MUST 记录实际包列表、扫描结果和替代验证命令
- **AND** 系统 MUST NOT 为了满足迁移任务而新增旧工具输出格式、旧 CLI flag 或旧文件路径兼容断言

#### Scenario: 多个独立工具输出差异
- **WHEN** 单个工具测试需要在一次执行中检查多个独立输出字段、文件内容差异或生成路径差异，且后续检查不依赖前置检查成功
- **THEN** 测试 MAY 使用 `testify/assert` 收集这些独立断言失败
- **AND** 初始化、命令执行、文件读取或解析失败 MUST 使用 `require` 立即终止当前测试

### Requirement: Go 测试按维护主题组织

认证与 provider 相关 Go 测试 MUST 按可独立维护的行为主题组织文件，避免单个测试文件长期承载多个不相关子主题。测试拆分 MUST 保持原有业务断言覆盖，不得通过删除关键场景降低覆盖范围。

#### Scenario: auth Redis session store 测试拆分

- **WHEN** 协作者维护 `user-service/internal/features/auth/infrastructure/redis` 的 session store 测试
- **THEN** token version cache、token version validator、refresh session 创建查询删除、refresh session rotation、全量 session 删除、purge pool/Fx lifecycle 和 Redis key schema 测试 MUST 分布在按主题命名的 `_test.go` 文件中
- **AND** 旧的跨主题大型 `session_store_test.go` MUST NOT 继续承载这些全部场景

#### Scenario: auth command use case 测试拆分

- **WHEN** 协作者维护 `user-service/internal/features/auth/application/command` 的 command use case 测试
- **THEN** login、change-password、refresh、logout current、logout all 和共享构造 helper MUST 分布在按主题命名的 `_test.go` 文件中
- **AND** 旧的跨 use case 大型 `service_test.go` MUST NOT 继续承载这些全部场景

#### Scenario: provider routes 与 Gin engine 测试拆分

- **WHEN** 协作者维护 `user-service/internal/providers` 的 routes 或 Gin engine 测试
- **THEN** auth middleware、route 注册冲突、tracing、request ID、HTTP metrics、panic recovery 和 runtime endpoint skip 测试 MUST 分布在按主题命名的 `_test.go` 文件中
- **AND** 单个 provider 测试文件 MUST NOT 同时承载所有 route、metrics、tracing 和 panic 场景

#### Scenario: 拆分后保持测试集合完整

- **WHEN** 大型测试文件被拆分
- **THEN** 协作者 MUST 对比拆分前后的 `Test` 函数集合或等价测试清单
- **AND** 目标包 `go test` MUST 通过

### Requirement: 复杂测试替身使用生成 mock

Go 测试中表示外部 collaborator port 调用契约的复杂 fake、stub 或 spy MUST 使用包内 `mockgen` 生成物替代。仅用于构造领域值、提供无行为分支统计快照、真实 miniredis/localcache 夹具或简单不可变配置的测试 helper MAY 保留在 `_test.go` 文件内。

#### Scenario: collaborator 调用契约使用 mockgen

- **WHEN** 测试需要断言 credential store、token issuer、refresh session store、token version store/cache、RBAC seed service、authorizer、watcher 或 metrics collaborator 的调用、参数、顺序或失败路径
- **THEN** 测试 MUST 使用同包或同 feature 测试包内的 `go.uber.org/mock/mockgen` 生成 mock 设置 expectation
- **AND** 测试 MUST NOT 通过复杂手写 fake/stub/spy 字段隐藏这些 collaborator 调用契约

#### Scenario: mock 生成入口归属

- **WHEN** 新增或替换测试 collaborator mock
- **THEN** `mock_generate.go` MUST 位于消费该 mock 的包或 feature-local 测试边界内
- **AND** 生成 mock MUST NOT 放入全局 `mocks/` 包或跨 feature 共享 mock 包
- **AND** 生成入口 MUST 使用可复现的 `go tool mockgen` 或仓库约定等价入口

#### Scenario: 允许轻量测试 helper

- **WHEN** 测试 helper 只构造领域对象、返回固定 stats、运行真实 workerpool task、包装 miniredis、包装真实 localcache 或提供无外部调用契约的配置值
- **THEN** helper MAY 保留为 `_test.go` 内部类型或函数
- **AND** helper MUST NOT 替代 mockgen 记录外部 port 调用、失败注入或调用顺序

### Requirement: Metrics no-op 生成约定一致

feature-local 业务 metrics interface 的 no-op 实现 MUST 继续通过业务中立生成器或统一生成约定维护。`common/runtime/observability/metrics` MUST 只承载生成器和通用 runtime metrics 能力，不得承载 user-service feature 的业务 metrics 方法。

#### Scenario: feature-local no-op 生成

- **WHEN** auth、permission 或其他 feature 定义 `Metrics` interface 且需要默认空实现
- **THEN** feature MUST 通过统一的 `nopgen` 生成约定生成 `metrics_nop_gen.go` 或等价 no-op 生成物
- **AND** no-op 生成物 MUST 与 feature-local `Metrics` interface 编译匹配

#### Scenario: common 不承载业务指标方法

- **WHEN** 统一 metrics no-op 生成约定被调整
- **THEN** `common/runtime/observability/metrics` MUST NOT 定义 auth 登录、refresh、logout、session purge、RBAC policy reload、watcher 或 route diff 等 user-service 业务指标方法
- **AND** auth/permission 业务指标方法 MUST 保留在所属 feature 边界内

#### Scenario: 生成物 drift 可验证

- **WHEN** metrics interface 或 mock 源 interface 变化
- **THEN** 仓库生成与完整验证流程 MUST 能更新对应生成物
- **AND** 未同步生成物 MUST 通过 `git diff --exit-code` 或等价 drift 检查暴露

### Requirement: 仓库级 OpenAPI 转换工具测试验证

系统 MUST 为 `tools/openapi-convert` 提供默认可执行的 Go 测试，覆盖 CLI 参数解析、错误路径和文件生成结果。根 `make test` 和 `make verify` MUST 执行该工具模块测试，工具模块测试失败时完整验证 MUST 失败。

#### Scenario: 根测试覆盖 OpenAPI 转换工具
- **WHEN** 协作者执行 `make test`
- **THEN** 系统 MUST 执行 `tools/openapi-convert` 模块的 Go 测试
- **AND** 系统 MUST 同时保持 `common` 和 `user-service` 模块测试执行

#### Scenario: 完整验证覆盖 OpenAPI 转换工具
- **WHEN** 协作者执行 `make verify`
- **THEN** 系统 MUST 通过测试阶段执行 `tools/openapi-convert` 模块测试
- **AND** 工具模块测试失败 MUST 阻止 `make verify` 成功完成

#### Scenario: CLI 参数错误回归测试
- **WHEN** `tools/openapi-convert` 测试覆盖缺少必填 `input`、`json`、`yaml` 或 `go` 输出路径的调用
- **THEN** 测试 MUST 断言 CLI 返回失败结果并输出明确错误

#### Scenario: root path 参数约束回归测试
- **WHEN** `tools/openapi-convert` 调用设置 `root-path` 但未设置 `root-server`
- **THEN** CLI MUST 返回失败结果
- **AND** 测试 MUST 断言该约束错误被保留

#### Scenario: 文件生成回归测试
- **WHEN** `tools/openapi-convert` 使用合法 Swagger 2 输入和输出路径执行
- **THEN** 测试 MUST 断言 JSON、YAML 和 Go embed 输出文件被创建
- **AND** 测试 MUST 断言生成内容包含 OpenAPI 版本、路径或 Go package 等关键结构

#### Scenario: 输入输出错误回归测试
- **WHEN** `tools/openapi-convert` 收到不存在的输入文件或不可写输出目标
- **THEN** CLI MUST 返回失败结果
- **AND** 测试 MUST 断言错误信息能定位输入转换或输出写入阶段

