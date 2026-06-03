## MODIFIED Requirements

### Requirement: Support safe enum validation

系统必须提供安全的 `enum` 自定义校验规则，用于校验实现 `IsValid() bool` 的枚举类型；当枚举类型提供允许值列表时，校验失败明细必须包含可读的允许取值提示。

#### Scenario: Accept valid enum
- **Given** DTO 字段使用 `validate:"enum"` 且字段值实现 `IsValid() bool`
- **When** `IsValid()` 返回 true
- **Then** 系统必须判定该字段校验通过

#### Scenario: Reject invalid enum
- **Given** DTO 字段使用 `validate:"enum"` 且字段值实现 `IsValid() bool`
- **When** `IsValid()` 返回 false
- **Then** 系统必须判定该字段校验失败

#### Scenario: Report allowed enum values when available
- **Given** DTO 字段使用 `validate:"enum"` 且字段值实现 `IsValid() bool`
- **Given** 该枚举类型提供固定顺序的允许值列表
- **When** `IsValid()` 返回 false
- **Then** 字段级校验错误消息必须为 `{字段名}取值不合法，允许值为：{值1}、{值2}、{值3}`

#### Scenario: Fall back when enum values are unavailable
- **Given** DTO 字段使用 `validate:"enum"` 且字段值未提供允许值列表
- **When** 系统生成 enum 校验失败消息
- **Then** 字段级校验错误消息必须为 `{字段名}取值不合法`

#### Scenario: Reject misconfigured enum without panic
- **Given** DTO 字段使用 `validate:"enum"` 但字段值未实现枚举接口或为 nil 指针
- **When** 系统执行 enum 校验
- **Then** 系统必须判定该字段校验失败
- **Then** 系统不得 panic
