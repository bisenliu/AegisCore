## Purpose

定义 AegisCore 仓库级 OPSX/OpenSpec 基础框架，使代理和协作者可以通过固定目录、配置、能力地图、主规格和变更工作流推进后续 change。

## Requirements

### Requirement: OPSX 基础目录结构

系统 MUST 提供可直接用于 OPSX/OpenSpec 协作的基础目录结构，包含代理入口文档、项目文档、OPSX 工作流文档、仓库级 OpenSpec 配置和主规格目录。

#### Scenario: 创建完整基础目录

- **WHEN** 协作者查看仓库根目录
- **THEN** 系统 MUST 提供 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/PRODUCT.md`、`docs/TESTING.md`、`docs/opsx/CAPABILITY_MAP.md`、`docs/opsx/CHANGE_WORKFLOW.md`、`openspec/config.yaml` 和 `openspec/specs/`

#### Scenario: README 入口可解析

- **WHEN** 协作者根据 `README.md` 中的 OPSX 入口访问文件
- **THEN** 每个被 README 引用的 OPSX/OpenSpec 文档路径 MUST 存在并包含可执行说明

#### Scenario: 不创建业务实现

- **WHEN** OPSX 基础框架被创建
- **THEN** 系统 MUST NOT 修改 Go 业务代码、数据库 migration、OpenAPI 生成物或部署清单的运行时行为

### Requirement: 仓库架构来源

系统 MUST 将 `AGENTS.md`、`docs/ARCHITECTURE.md` 和 `openspec/specs/` 作为当前有效结构、分层和能力规格来源，保证代理和协作者围绕同一组规则工作。

#### Scenario: 判断结构规则

- **WHEN** 协作者需要判断目录归属、feature 分层或共享边界
- **THEN** 系统 MUST 在 `AGENTS.md`、`docs/ARCHITECTURE.md` 和相关主规格中提供一致规则，并避免把临时目录草稿作为正式来源

#### Scenario: Go workspace 模块归属

- **WHEN** 协作者需要构建、测试、lint 或生成代码
- **THEN** 系统 MUST 通过根 `Makefile` 入口或在 `common/`、`user-service/` 模块目录内执行模块级命令，且 MUST NOT 把仓库根目录当作单一 Go module

#### Scenario: 服务私有命令命名

- **WHEN** 单个服务的 Makefile 目标暴露在仓库根目录
- **THEN** 目标名 MUST 带服务名前缀，例如 `user-service-seed-rbac`，避免使用缺少服务上下文的根目标名

### Requirement: 仓库级 OpenSpec 配置

系统 MUST 在 `openspec/config.yaml` 中声明仓库级上下文和 artifact 规则，使后续 `/opsx:*` 产出遵循 AegisCore 的语言、技术栈、分层和验证约束。

#### Scenario: 中文输出规则生效

- **WHEN** 代理读取 OpenSpec instructions 创建 proposal、design、specs 或 tasks
- **THEN** 产出正文 MUST 使用简体中文，技术术语、路径、命令、配置项名称和 Go symbol MAY 保留英文原文

#### Scenario: 技术栈和边界可见

- **WHEN** 代理读取 `openspec/config.yaml`
- **THEN** 配置 MUST 明确 AegisCore 使用 Go workspace、Gin、Fx、Ent、Atlas、PostgreSQL、Redis、Casbin、Prometheus、OpenTelemetry、Docker、Kubernetes 和 Helm，并说明 `common/`、`user-service/`、`deployments/` 的职责边界

#### Scenario: artifact 规则可执行

- **WHEN** 新 change 生成 artifacts
- **THEN** proposal、specs、design 和 tasks MUST 分别遵循配置中定义的能力命名、场景格式、影响分析和验证任务规则

#### Scenario: OpenSpec 任务完成门禁

- **WHEN** 协作者执行 OpenSpec change 的 tasks 并准备将任务或 change 标记为完成
- **THEN** 系统 MUST 要求相关单元测试、`make lint` 和 `make verify` 全部通过
- **AND** 任一验证未通过或未运行时，该任务或 change MUST NOT 被视为完成

#### Scenario: 归并既有能力

- **WHEN** 新需求可以落入现有主规格表达的稳定 capability
- **THEN** change MUST 优先更新既有 `openspec/specs/<capability>/spec.md` 或对应 delta，避免按旧目录名、临时任务名或一次性重构名创建重复 spec

### Requirement: 代理导航文档

系统 MUST 提供 `AGENTS.md` 作为 AI 代理和协作者的快速导航图，并将详细说明链接到 `docs/` 与 `openspec/`。

#### Scenario: 新代理进入仓库

- **WHEN** 新代理首次处理 AegisCore 任务
- **THEN** `AGENTS.md` MUST 指向架构、开发、产品、测试、能力地图、OPSX 工作流和主规格入口

#### Scenario: 代理入口包含关键边界

- **WHEN** 代理需要修改 `common/`、feature、`internal/shared`、`internal/integration`、Ent migration、OpenAPI 或部署资产
- **THEN** `AGENTS.md` MUST 提供可扫读的关键入口、高风险区域、架构边界和验证命令

#### Scenario: 变更入口清晰

- **WHEN** 代理准备处理跨 feature、跨模块、外部契约、schema、部署或行为变更
- **THEN** `AGENTS.md` MUST 要求先确认 capability，再使用 `/opsx:explore`、`/opsx:propose` 或 `/opsx:apply`

#### Scenario: 高风险边界被提示

- **WHEN** 代理计划修改认证、RBAC、migration、OpenAPI、共享契约或部署观测资产
- **THEN** `AGENTS.md` MUST 提示相关路径、验证命令和需要同步更新的文档或规格

### Requirement: 能力地图

系统 MUST 提供 `docs/opsx/CAPABILITY_MAP.md`，把稳定 capability、主要代码位置、主规格路径和状态连接起来。

#### Scenario: 查找能力归属

- **WHEN** 协作者需要判断一个需求属于哪个能力
- **THEN** 能力地图 MUST 能通过 capability 表格定位业务说明、主要代码位置和对应 `openspec/specs/<capability>/spec.md`

#### Scenario: 记录交叉依赖

- **WHEN** 一个 capability 同时依赖 `common/`、`user-service/` 和 `deployments/`
- **THEN** 能力地图 MUST 说明关键入口和交叉依赖，避免后续 change 只修改单一路径而遗漏契约或观测影响

#### Scenario: 标注待补能力

- **WHEN** 仓库存在尚未细化成主规格的能力
- **THEN** 能力地图 MUST 标注待补能力和建议拆分方向

### Requirement: OPSX 变更工作流说明

系统 MUST 提供 `docs/opsx/CHANGE_WORKFLOW.md`，说明本仓库如何使用 `/opsx:explore`、`/opsx:propose`、`/opsx:apply`、`/opsx:verify` 和 `/opsx:archive`。

#### Scenario: 提出新变更

- **WHEN** 协作者需要提出跨模块或行为变更
- **THEN** 工作流文档 MUST 指导其选择 kebab-case change name，生成 proposal、design、specs 和 tasks，并保持中文 artifact 内容

#### Scenario: 准备实现

- **WHEN** proposal、design、specs 和 tasks 已完成
- **THEN** 工作流文档 MUST 指导协作者运行 `/opsx:apply <change-name>` 并按 tasks 执行实现与验证

#### Scenario: 完成归档

- **WHEN** change 已实现、验证并合并
- **THEN** 工作流文档 MUST 指导协作者使用 `/opsx:archive <change-name>` 将 delta specs 合并回主规格
