## Purpose

定义 AegisCore 仓库级 OPSX/OpenSpec 基础框架，使代理和协作者能够通过统一入口、能力地图、主规格和变更工作流推进后续 change。

## Requirements

### Requirement: OPSX 正式入口与规范来源

系统 MUST 提供完整且可导航的 OPSX/OpenSpec 基础结构，并明确根导航、架构文档、能力地图、OpenSpec 配置和主规格的职责边界。

#### Scenario: 治理入口完整且引用可解析

- **WHEN** 协作者从仓库根目录或 `README.md` 进入 OPSX 工作流
- **THEN** `AGENTS.md`、核心 `docs/` 文档、`docs/opsx/CAPABILITY_MAP.md`、`docs/opsx/CHANGE_WORKFLOW.md`、`openspec/config.yaml` 和 `openspec/specs/` MUST 存在
- **AND** 被引用路径 MUST 可解析，并 MUST 指向相关 capability、OPSX 命令、风险边界和验证入口

#### Scenario: 规范来源职责明确且一致

- **WHEN** 协作者判断目录归属、feature 分层、共享边界、高风险区域或稳定 capability 行为
- **THEN** `AGENTS.md` MUST 提供代理导航和快速规则摘要，`docs/ARCHITECTURE.md` MUST 提供代码结构和分层边界说明，`openspec/specs/` MUST 提供稳定 capability 行为规范
- **AND** `docs/opsx/CAPABILITY_MAP.md` MUST 作为归属索引而不重复定义行为，`openspec/config.yaml` MUST 定义 artifact 生成规则和代理上下文
- **AND** 临时目录草稿 MUST NOT 被视为正式架构或能力来源

### Requirement: OpenSpec 配置与能力治理

系统 MUST 通过 `openspec/config.yaml` 和 `docs/opsx/CAPABILITY_MAP.md` 统一声明仓库上下文、artifact 规则、稳定 capability、代码归属、主规格和交叉依赖。

#### Scenario: artifact 语言、结构和可验证性符合配置

- **WHEN** 代理创建 proposal、design、spec delta 或 tasks
- **THEN** 正文 MUST 使用简体中文，技术术语、路径、命令、配置项和 Go symbol MAY 保留英文原文
- **AND** artifact MUST 遵循 capability 命名、场景格式、影响分析、架构边界和验证任务规则
- **AND** 每个 Requirement MUST 至少包含一个可验证 Scenario

#### Scenario: 能力定位与去重

- **WHEN** 协作者评估一个需求
- **THEN** 能力地图 MUST 能定位其业务说明、主要代码位置、主规格、状态及关键交叉依赖
- **AND** 尚未独立建模的候选能力 MUST 标注当前归属和拆分条件
- **AND** 新需求可以由现有稳定 capability 表达时，change MUST 优先更新现有主规格或对应 delta
- **AND** 系统 MUST NOT 按旧目录名、临时任务名或一次性重构名创建重复 capability

#### Scenario: 纯 OPSX 治理调整不改变运行时

- **WHEN** 仅维护 OPSX 配置、导航或能力地图
- **THEN** 系统 MUST NOT 改变 Go 业务代码、数据库 migration、OpenAPI 生成物或部署清单的运行时行为
- **AND** 若变更需要改变业务、数据、API 或部署运行时语义，MUST 归入对应 capability 的 change

### Requirement: OPSX change 生命周期与完成门禁

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

#### Scenario: 归档回主规格

- **WHEN** change 已实现并通过验证
- **THEN** 工作流 MUST 指导协作者使用 `/opsx:archive <change-name>` 将 delta 合并回主规格
- **AND** 归档后主规格、能力地图和文档入口 MUST 保持一致
