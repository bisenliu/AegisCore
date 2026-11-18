## Purpose

定义 AegisCore 仓库级 OPSX/OpenSpec 治理入口、能力归属、change 生命周期和服务级 provider 物理边界。

## Requirements

### Requirement: OPSX 治理入口、artifact 与 change 生命周期

仓库 MUST 通过 `AGENTS.md`、核心架构文档、`openspec/config.yaml`、能力地图、变更工作流和主规格提供单一且可导航的治理结构。稳定行为变更 MUST 归入明确 capability，并通过一致的 proposal、design、spec delta、tasks、实施、验证与归档流程管理。

#### Scenario: 导航与权威来源一致

- **WHEN** 协作者从仓库根目录进入 OPSX 工作流或判断代码归属
- **THEN** `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md`、`docs/opsx/CHANGE_WORKFLOW.md`、`openspec/config.yaml` 和 `openspec/specs/` MUST 存在且引用路径可解析
- **AND** `AGENTS.md` MUST 提供导航与规则摘要，架构文档 MUST 定义结构边界，主规格 MUST 定义稳定行为，能力地图 MUST 只提供 capability、代码位置、状态和依赖索引
- **AND** 临时草稿、已完成 change 或一次性任务名 MUST NOT 取代当前主规格或创建重复 capability
- **AND** 已合并的 change MUST 从 `openspec/changes/` 删除，仓库 MUST NOT 保留 `openspec/changes/archive/`

#### Scenario: artifact 语言、结构与能力定位

- **WHEN** 代理创建或更新 proposal、design、spec delta 或 tasks
- **THEN** 正文 MUST 使用简体中文，技术术语、路径、命令、配置项和 Go symbol MAY 保留英文原文
- **AND** 每个 Requirement MUST 至少包含一个具有明确 `WHEN` 与 `THEN` 的可验证 Scenario
- **AND** 新需求可由现有 capability 表达时 MUST 更新该 capability，不得按旧目录名或临时重构名创建替代规格

#### Scenario: 提案、实施与归档

- **WHEN** 需求改变跨模块、外部契约、schema、部署或其他长期稳定行为
- **THEN** 协作者 MUST 使用 kebab-case change name 创建相互一致的 proposal、design、spec delta 和 tasks，并在 artifacts 就绪后实施
- **WHEN** change 已实现
- **THEN** 预期变更 MUST 先被暂存，相关测试、`make lint` 和 `make verify` MUST 全部通过，tasks MUST 全部完成，之后才可将 delta 合并回主规格并删除 change 目录
- **AND** 任一必要验证未运行或未通过时，change MUST NOT 被视为完成

#### Scenario: 纯治理调整不改变运行时

- **WHEN** 仅维护 OPSX 配置、导航、能力地图或无行为变化的主规格校准
- **THEN** 变更 MUST NOT 改变业务代码、数据库 migration、OpenAPI 生成物或部署清单的运行时语义
- **AND** 一旦需要改变运行时语义，变更 MUST 归入对应 capability 的 OpenSpec change

#### Scenario: 代码注释质量约束

- **WHEN** 协作者新增或修改 Go 导出类型、函数、方法、接口、常量、变量或包
- **THEN** 对应文档注释 MUST 以被注释对象名称开头，并准确描述用途、行为约束、重要输入输出和错误语义
- **AND** 已有注释缺失、不完整或与实现不一致时 MUST 同步补充或修正
- **WHEN** 实现包含复杂逻辑、关键分支、边界条件、异常处理、并发操作、性能敏感路径或不易理解的权衡
- **THEN** 代码 MUST 添加必要且简洁的行内注释，说明原因、约束或风险，而不是重复代码表面含义
- **AND** 能通过更清晰命名、职责拆分或简化控制流解决的问题 MUST 优先改进实现，不得用注释掩盖不清晰代码
- **AND** 代码注释和函数/方法注释 MUST 使用中文，日志消息 MUST 使用英文，日志字段名 MUST 使用稳定英文 `snake_case`

### Requirement: 服务级 provider 物理边界治理

`user-service/internal/providers` 根包 MUST 只汇总服务级 Fx module，具体 datastore、observability、security 和 transport 接线 MUST 位于对应子包，且不得通过兼容 wrapper、alias 或重复构造器绕过该边界。

#### Scenario: provider 根包与子包职责

- **WHEN** 协作者查看或修改服务级 provider
- **THEN** 根包 MUST 只维护 `WiringModule`、`RuntimeModule`、`Module` 及必要测试
- **AND** PostgreSQL、Redis、Ent 及其插件 MUST 位于 `providers/datastore`
- **AND** datastore constructor MUST 接收调用方传入的资源名，MUST NOT 写死资源名，也 MUST NOT 遍历配置 map 自动供应未登记资源
- **AND** health、metrics 和 tracing 接线 MUST 位于 `providers/observability`
- **AND** JWT、认证 token policy 和 password service 接线 MUST 位于 `providers/security`
- **AND** Gin、routes 和 API rate limiter 接线 MUST 位于 `providers/transport`

#### Scenario: datastore 按显式登记的资源名供应连接

- **WHEN** composition root 以任意资源名登记一个 PostgreSQL 或 Redis 连接
- **THEN** datastore constructor MUST 只供应该资源名的连接
- **AND** 未登记的资源名 MUST NOT 被供应，即使配置 map 中存在该资源
- **AND** 只有与本服务 Ent schema 相同的连接才由 composition root 登记 Ent client
- **AND** feature 选择具体资源名 MUST 留在 feature composition，不得回写到 datastore constructor

#### Scenario: 文档和测试跟随边界

- **WHEN** provider 目录或职责变化
- **THEN** `docs/ARCHITECTURE.md` 与能力地图 MUST 同步更新，测试 MUST 跟随被测关注点
- **AND** 生产代码 MUST NOT 为测试便利新增冗余 API、全局替身或兼容分支
