## ADDED Requirements

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
