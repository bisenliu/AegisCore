# project-naming-consistency

## Purpose

项目命名一致性能力定义仓库命名审查、低风险重命名边界、引用同步和结果报告要求，确保命名标准化不改变现有外部契约或功能行为。

## Requirements

### Requirement: Repository naming audit
系统 SHALL 支持对仓库目录名、文件名、Go 包名、类型名、函数名、方法名、OpenSpec capability 名和文档命名进行一致性审查，并将发现项按外部契约、公共 Go API、内部 Go API、文档/规格表达、工具链或迁移历史分类。

#### Scenario: Naming audit classifies candidates
- **WHEN** 实施命名标准化前审查仓库命名
- **THEN** 每个候选命名问题 MUST 标注影响范围、所属 capability、建议处理方式和兼容性风险

#### Scenario: Naming audit avoids unsupported assumptions
- **WHEN** 命名候选涉及尚未实现或未规格化的能力
- **THEN** 审查结果 MUST 不承诺新增业务能力或改变现有功能行为

### Requirement: Non-functional rename boundary
系统 SHALL 只在不改变外部可观察行为的前提下执行命名标准化，且 MUST 保留已成为外部契约、工具链约定或迁移历史的名称。

#### Scenario: External contract name is encountered
- **WHEN** 候选改名涉及 HTTP 路径、JSON 字段、响应码数值、header 名称、配置 key、环境变量、Go module path 或服务启动命令
- **THEN** 实现 MUST 保留该名称，或将其标记为需要单独 breaking change 的后续事项

#### Scenario: Migration history name is encountered
- **WHEN** 候选改名涉及已存在 Atlas migration 文件名或校验历史
- **THEN** 实现 MUST 不重命名该 migration 文件或修改迁移历史

### Requirement: Reference synchronization
系统 SHALL 在执行低风险命名修改后同步更新所有引用点，包括 Go imports、函数调用、测试、文档、OpenSpec 规格和 capability map。

#### Scenario: Internal Go symbol is renamed
- **WHEN** 内部 Go 类型、函数、方法、变量或包名被重命名
- **THEN** 所有 workspace 内引用 MUST 同步更新，且相关 Go 模块测试 MUST 通过

#### Scenario: Documentation name is corrected
- **WHEN** 文档或规格中的 capability、响应码或边界名表达被修正
- **THEN** 相关文档和 OpenSpec 文件 MUST 与当前真实代码行为保持一致

### Requirement: Naming result reporting
系统 SHALL 在实现完成后输出修改清单，并说明每一处命名修改的原因、影响范围和是否改变外部行为。

#### Scenario: Rename report is produced
- **WHEN** 命名标准化实现完成
- **THEN** 输出 MUST 列出每个被修改的目录、文件、函数、类型、变量、文档或规格名称及修改原因

#### Scenario: Retained risky names are reported
- **WHEN** 审查发现不建议修改的高风险名称
- **THEN** 输出 MUST 说明保留原因和后续如需修改应采用的变更类型
