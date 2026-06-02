## ADDED Requirements

### Requirement: Classify reusable enum constants
系统 SHALL 将项目中的枚举型类型、公共常量和重复契约值按归属分类，只有跨模块、跨服务或跨公共能力复用的契约值才能迁移到 `common` 模块。

#### Scenario: Identify common reusable constants
- **WHEN** 实现审查项目中的 `const` 组、enum-like 类型和重复字符串值
- **THEN** 跨服务运行时资源名、认证协议边界值、标准响应码、公共响应消息和共享 validation 契约 MUST 归类为 common 可复用常量
- **THEN** 实现 MUST 记录这些常量的当前位置、目标位置和引用更新范围

#### Scenario: Keep service business constants local
- **WHEN** 实现审查用户服务业务错误文案或用户域专属状态
- **THEN** 仅服务内使用且包含用户业务语义的常量 MUST 保留在 `user-services` 内
- **THEN** `common` MUST NOT 引入用户资料、登录凭据或用户会话业务文案，除非存在独立共享业务错误文案能力

#### Scenario: Do not migrate generated constants
- **WHEN** 实现审查 `user-services/ent/` 下的生成常量
- **THEN** 实现 MUST NOT 手写、移动或重命名这些生成常量
- **THEN** 如未来需要改变 Ent 表名、字段名或生成枚举，MUST 修改 Ent schema 并运行生成流程

### Requirement: Preserve external contract values during enum migration
系统 SHALL 在公共枚举/常量迁移时保持所有外部可观察契约值不变。

#### Scenario: Preserve API response contracts
- **WHEN** 响应码、响应消息或认证失败文案的常量来源被迁移到 `common`
- **THEN** HTTP 响应 envelope 字段 MUST 保持 `success`、`code`、`message`、`data` 和既有错误字段约定
- **THEN** 标准业务码数字值和 HTTP status 映射 MUST 保持不变

#### Scenario: Preserve authentication boundary values
- **WHEN** Bearer token type、Authorization header 或 Bearer prefix 的常量来源被整合
- **THEN** HTTP header 名 MUST 仍为 `Authorization`
- **THEN** Bearer token type MUST 仍为 `Bearer`
- **THEN** Authorization header prefix MUST 仍为 `Bearer `

#### Scenario: Preserve runtime resource identities
- **WHEN** Redis、PostgreSQL 或 Ent runtime dependency 名称迁移为 common 常量
- **THEN** 运行时资源名 MUST 仍使用 `user_db`、`common_db` 和 `cache_redis`
- **THEN** YAML/env 配置路径、Fx 注入名称和日志中的依赖名称语义 MUST 保持不变

### Requirement: Report migration inventory and compatibility notes
系统 SHALL 在实现完成后提供迁移清单、影响范围和兼容性说明，帮助维护者确认哪些常量已迁移、哪些常量明确不迁移。

#### Scenario: Summarize migrated constants
- **WHEN** 公共枚举/常量迁移实现完成
- **THEN** 输出 MUST 列出已迁移或整合的常量、原位置、目标位置和主要引用更新文件
- **THEN** 输出 MUST 列出已确认不迁移的常量类别及原因

#### Scenario: Summarize compatibility concerns
- **WHEN** 公共枚举/常量迁移实现完成
- **THEN** 输出 MUST 说明 HTTP API、错误码、header、配置 key、Fx name、数据库 schema 和生成代码兼容性影响
- **THEN** 如存在 Go struct tag 仍需保留字符串字面量，输出 MUST 明确说明原因
