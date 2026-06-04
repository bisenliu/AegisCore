## ADDED Requirements

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
