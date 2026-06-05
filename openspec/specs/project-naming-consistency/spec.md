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

### Requirement: Structure Go package naming review feedback
系统 SHALL 在整理 Go 包命名类代码评审意见时，使用中文分别给出问题说明、原因分析和建议改法，并以 Go 包命名最佳实践作为判断依据。反馈 MUST 明确包名应短小、全小写、语义清晰，缩写词不得使用混合大小写或不符合 Go 习惯的风格。

#### Scenario: Explain package abbreviation naming issue
- **WHEN** 评审意见涉及 `user-services/internal/errmsg/` 或类似错误消息包命名
- **THEN** 反馈 MUST 在问题说明中指出 Go package name 不应使用混合大小写、下划线或冗长表达
- **THEN** 反馈 MUST 在原因分析中说明 Go 包名通常使用短小全小写名称，调用方会通过 `package.Identifier` 阅读语义，包名无需重复过多上下文
- **THEN** 反馈 MUST 在建议改法中给出全小写、短小且语义明确的命名建议，例如 `errmsg`

#### Scenario: Avoid changing external behavior in naming feedback
- **WHEN** 包命名评审反馈给出改名建议
- **THEN** 反馈 MUST 说明后续真正改名时需要同步 Go imports、测试和相关文档引用
- **THEN** 反馈 MUST NOT 承诺改变 HTTP API、错误码、响应信封、配置 key 或数据库 schema

### Requirement: Use Username spelling for internal user name symbols
系统 SHALL 在内部 Go 符号中使用 `Username` 表达用户名领域概念，不得将该领域术语拆分为 `UserName` 或 `userName`。实现命名修正时 MUST 同步更新 workspace 内引用，并 MUST 保持外部可观察契约不变。

#### Scenario: Rename internal username error message constants
- **WHEN** 用户服务错误消息常量表达用户名无效或用户名为空语义
- **THEN** 常量名 MUST 使用 `Username` 单词边界
- **THEN** 常量名 MUST NOT 使用 `UserName` 拆词风格
- **THEN** 所有 workspace 内引用 MUST 同步更新

#### Scenario: Preserve external username contracts during rename
- **WHEN** 命名修正涉及用户名领域术语
- **THEN** 实现 MUST 保留 `username` JSON 字段、查询参数、数据库字段和配置字符串不变
- **THEN** 实现 MUST 保留现有公开错误消息文本、HTTP 状态码、响应码和响应信封语义不变

### Requirement: Name user query controller handler by external user ID

系统 SHALL 要求用户资料查询 controller handler 使用外部用户 ID 语义命名。对于处理 `GET /api/v1/users/:user_id` 的 controller 方法，内部 Go 方法名 MUST 明确包含 `UserID`，避免与内部数据库自增 `id` 查询语义混淆。命名标准化 MUST 同步更新所有 workspace 内 Go 引用，并保持外部可观察契约不变。

#### Scenario: Rename user query controller handler
- **WHEN** `UserController` 处理 `GET /api/v1/users/:user_id` 查询用户资料请求
- **THEN** 对应 handler 方法名 MUST 为 `GetByUserID`
- **THEN** 代码中 MUST NOT 继续使用 `UserController.GetByID` 表达该外部 UUID 查询 handler

#### Scenario: Preserve user query external contract during handler rename
- **WHEN** 用户查询 controller handler 命名标准化完成
- **THEN** `GET /api/v1/users/:user_id` 路径 MUST 保持不变
- **THEN** `user_id` 路径参数名、响应 envelope、公开 JSON 字段、业务错误码和认证要求 MUST 保持不变
- **THEN** 实现 MUST NOT 修改数据库 schema、Atlas migration 或内部自增 `id` 字段语义
