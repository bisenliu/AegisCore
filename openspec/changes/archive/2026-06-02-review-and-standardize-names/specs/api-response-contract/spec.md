## ADDED Requirements

### Requirement: Response naming documentation remains compatible
响应契约相关命名标准化 SHALL 统一规格和文档中的响应码命名表达，但不得改变响应信封字段、响应 `code` 数值、错误映射、validation error details 或 panic recovery 响应行为。

#### Scenario: Response code names are normalized in specs
- **WHEN** 实现修正规格或文档中的响应码名称表达
- **THEN** 规格 MUST 明确对外响应 `code` 仍使用当前数字枚举，且 Go 常量名或语义标签不得改变 JSON payload 结构

#### Scenario: Response envelope is preserved
- **WHEN** 命名标准化涉及 `common/response` 或 controller 响应引用
- **THEN** HTTP 响应 MUST 继续使用 `success`、`code`、`message`、`data` 和既有错误字段约定
