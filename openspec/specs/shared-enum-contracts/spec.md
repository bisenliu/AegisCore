# shared-enum-contracts

## Purpose

共享枚举契约能力定义跨模块、跨服务或跨公共能力复用的枚举型类型、公共常量和重复契约值的归属、迁移边界与兼容性要求。

## Requirements

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

### Requirement: Avoid unnecessary enum formatting overhead
系统 SHALL 在枚举允许值、枚举契约值或固定常量字符串化实现中避免使用不必要的通用格式化函数，但 MUST 保持既有契约值和可读性。

#### Scenario: Replace fixed enum value formatting
- **WHEN** 枚举字符串输出只包含编译期已知且外部契约稳定的数字或字符串常量
- **THEN** 实现 MUST 使用字符串字面量、`strconv` 或普通字符串拼接等更直接的方式替代不必要的 `fmt.Sprint` 或 `fmt.Sprintf`
- **THEN** 输出的枚举允许值 MUST 与变更前保持一致

#### Scenario: Keep readable semantic formatting
- **WHEN** `fmt.Sprintf` 或类似格式化函数用于错误消息、日志、复合模板、格式控制或调试输出
- **THEN** 实现 MUST 仅在替换后可读性不下降且输出完全一致时修改
- **THEN** 对未修改场景 MUST 在实现结果中说明保留依据

#### Scenario: Preserve enum contract compatibility
- **WHEN** 优化枚举字符串化或常量拼接实现
- **THEN** 枚举数字值、JSON/text 反序列化、API 响应、错误码、配置 key 和数据库 schema MUST 保持不变
