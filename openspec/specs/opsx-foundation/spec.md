## Purpose

定义 AegisCore 仓库级 OPSX/OpenSpec 基础框架，使代理和协作者能够通过统一入口、能力地图、主规格和变更工作流推进后续 change。

## Requirements

### Requirement: OPSX 基础结构与架构来源

系统 MUST 提供完整且可导航的 OPSX/OpenSpec 基础结构，并将 `AGENTS.md`、`docs/ARCHITECTURE.md` 和 `openspec/specs/` 作为当前有效的仓库结构、分层边界和能力规格来源。

#### Scenario: 基础入口完整

- **WHEN** 协作者从仓库根目录或 `README.md` 进入 OPSX 工作流
- **THEN** `AGENTS.md`、核心 `docs/` 文档、`docs/opsx/CAPABILITY_MAP.md`、`docs/opsx/CHANGE_WORKFLOW.md`、`openspec/config.yaml` 和 `openspec/specs/` MUST 存在，且被引用路径 MUST 可解析

#### Scenario: 架构规则一致

- **WHEN** 协作者判断目录归属、feature 分层、共享边界或高风险区域
- **THEN** `AGENTS.md`、`docs/ARCHITECTURE.md` 和相关主规格 MUST 提供一致规则及对应验证入口
- **AND** 临时目录草稿 MUST NOT 被视为正式架构来源

#### Scenario: Go workspace 与命令归属

- **WHEN** 协作者构建、测试、lint、生成代码或暴露服务私有 Makefile 目标
- **THEN** 操作 MUST 通过根 `Makefile` 或对应 module 执行，仓库根目录 MUST NOT 被当作单一 Go module
- **AND** 根目录中的服务私有目标 MUST 使用服务名前缀，例如 `user-service-seed-rbac`

#### Scenario: 导航关键变更

- **WHEN** 代理处理跨 feature、跨模块、外部契约、schema、部署、认证、RBAC、migration、OpenAPI 或观测变更
- **THEN** `AGENTS.md` MUST 指向相关路径、capability、OPSX 命令、风险边界和验证命令

### Requirement: OpenSpec 配置与能力地图

系统 MUST 通过 `openspec/config.yaml` 和 `docs/opsx/CAPABILITY_MAP.md` 统一声明仓库上下文、artifact 规则、稳定 capability、代码归属、主规格和交叉依赖。

#### Scenario: 生成中文且可验证的 artifact

- **WHEN** 代理创建 proposal、design、spec delta 或 tasks
- **THEN** 正文 MUST 使用简体中文，技术术语、路径、命令、配置项和 Go symbol MAY 保留英文原文
- **AND** artifact MUST 遵循 capability 命名、场景格式、影响分析、架构边界和验证任务规则

#### Scenario: 定位能力归属

- **WHEN** 协作者评估一个需求
- **THEN** 能力地图 MUST 能定位其业务说明、主要代码位置、主规格、状态及关键交叉依赖
- **AND** 尚未独立建模的候选能力 MUST 标注当前归属和拆分条件

#### Scenario: 归并既有能力

- **WHEN** 新需求可以由现有稳定 capability 表达
- **THEN** change MUST 优先更新现有主规格或对应 delta
- **AND** 系统 MUST NOT 按旧目录名、临时任务名或一次性重构名创建重复 capability

#### Scenario: OPSX 基础调整不改变运行时

- **WHEN** 仅维护 OPSX 配置、导航或能力地图
- **THEN** 系统 MUST NOT 改变 Go 业务代码、数据库 migration、OpenAPI 生成物或部署清单的运行时行为

### Requirement: OPSX 变更与完成工作流

系统 MUST 在 `docs/opsx/CHANGE_WORKFLOW.md` 中定义从探索、提案、实施、验证到归档的完整流程，并确保未通过必要验证的 change 不被视为完成。

#### Scenario: 提出并实施变更

- **WHEN** 协作者提出跨模块或稳定行为变更
- **THEN** 工作流 MUST 指导其确认 capability、使用 kebab-case change name，并创建一致的 proposal、design、spec delta 和 tasks
- **AND** artifacts 就绪后 MUST 通过 `/opsx:apply <change-name>` 按依赖顺序实施

#### Scenario: 完成验证门禁

- **WHEN** change 准备标记完成
- **THEN** 实现、规格和文档任务 MUST 全部完成，预期变更 MUST 先被暂存
- **AND** 相关单元测试、`make lint` 和 `make verify` MUST 全部通过
- **AND** 任一必要验证未运行或未通过时，任务或 change MUST NOT 被视为完成

#### Scenario: 注释和文档同步

- **WHEN** change 修改复杂业务逻辑、关键函数或边界条件
- **THEN** 必要代码注释和相关文档 MUST 同步更新并保持简洁准确
- **AND** 注释 MUST NOT 逐行复述显而易见的实现

#### Scenario: 归档回主规格

- **WHEN** change 已实现并通过验证
- **THEN** 工作流 MUST 指导协作者使用 `/opsx:archive <change-name>` 将 delta 合并回主规格
- **AND** 归档后主规格、能力地图和文档入口 MUST 保持一致
