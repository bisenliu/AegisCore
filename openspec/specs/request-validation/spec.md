# request-validation

## Purpose

请求校验能力为 Gin controller 提供统一的请求参数绑定、结构体校验、字段名解析、DTO 扩展钩子、自定义规则和校验失败响应映射，减少各服务重复实现请求校验逻辑。

## Requirements

### Requirement: Provide shared request validation

系统必须在 `common` 中提供共享请求校验能力，用于 Gin controller 绑定 URI、query、JSON 和 form 请求参数，并通过同一个 validator 实例执行结构体校验。

#### Scenario: Bind URI parameters
- **Given** controller 定义包含 `uri` 和 `validate` tag 的请求 DTO
- **When** controller 使用共享校验器绑定 Gin URI 参数
- **Then** 系统必须将 URI 参数写入 DTO
- **Then** 系统必须使用 DTO 的 `validate` tag 执行结构体校验

#### Scenario: Bind query parameters
- **Given** controller 定义包含 `form` 或 `query` tag 的请求 DTO
- **When** controller 使用共享校验器绑定 query 参数
- **Then** 系统必须将 query 参数写入 DTO
- **Then** 系统必须使用 DTO 的 `validate` tag 执行结构体校验

#### Scenario: Bind JSON body
- **Given** controller 定义包含 `json` 和 `validate` tag 的请求 DTO
- **When** controller 使用共享校验器绑定 JSON body
- **Then** 系统必须将 JSON body 写入 DTO
- **Then** 系统必须使用 DTO 的 `validate` tag 执行结构体校验

#### Scenario: Reject empty JSON body
- **Given** 请求需要 JSON body
- **When** 请求 body 为空
- **Then** 系统必须将该请求判定为校验失败
- **Then** controller 必须能够返回 HTTP 400 失败信封

### Requirement: Normalize validation errors

系统必须把 binding、结构体校验和业务自定义校验错误归一化为调用方可读的校验错误，并允许 controller 使用 `common/response.Envelope` 输出失败响应。

#### Scenario: Reject invalid field value
- **Given** 请求 DTO 字段包含 `validate:"required"`、`validate:"gt=0"` 或其他 validator tag
- **When** 请求字段不满足校验规则
- **Then** 系统必须返回可识别的校验错误
- **Then** 错误必须包含可读 message

#### Scenario: Reject JSON type mismatch
- **Given** 请求 DTO 字段期望整数、布尔、字符串、数组或对象类型
- **When** JSON body 中对应字段类型不匹配
- **Then** 系统必须返回可读的字段类型错误
- **Then** 错误消息不得暴露内部反射或 decoder 细节

#### Scenario: Preserve response envelope
- **Given** controller 使用共享校验器处理请求校验失败
- **When** controller 输出失败响应
- **Then** 响应必须使用 `common/response.Envelope`
- **Then** 响应必须为 HTTP 400 和 `BAD_REQUEST` 错误码，除非 controller 显式映射为已有兼容消息

### Requirement: Resolve display names from request DTO tags

系统必须从请求 DTO 的 tag 中解析字段显示名，用于生成稳定、可读的校验错误消息。

#### Scenario: Use label tag first
- **Given** DTO 字段同时包含 `label` 和请求绑定 tag
- **When** 系统生成校验错误消息
- **Then** 字段显示名必须优先使用 `label` tag

#### Scenario: Use request tag when label is absent
- **Given** DTO 字段不包含 `label` tag 但包含 `json`、`form`、`uri` 或 `query` tag
- **When** 系统生成校验错误消息
- **Then** 字段显示名必须使用对应请求 tag 的字段名

#### Scenario: Ignore omitted fields
- **Given** DTO 字段包含 `json:"-"` 或其他请求 tag 的 `-` 名称
- **When** 系统解析字段显示名
- **Then** 系统不得把该字段作为对外字段名输出

### Requirement: Support DTO extension hooks

系统必须支持请求 DTO 通过接口声明默认值填充和业务自定义校验逻辑。

#### Scenario: Apply defaults before validation
- **Given** 请求 DTO 实现 `SetDefaults()` 方法
- **When** binding 完成且结构体校验开始前
- **Then** 系统必须调用 `SetDefaults()`
- **Then** 默认值必须参与后续结构体校验

#### Scenario: Run custom validation after struct validation
- **Given** 请求 DTO 实现 `Validate() error` 方法
- **When** binding 和结构体 tag 校验均通过
- **Then** 系统必须调用 `Validate()`
- **Then** `Validate()` 返回错误时请求必须被判定为校验失败

### Requirement: Support safe enum validation

系统必须提供安全的 `enum` 自定义校验规则，用于校验实现 `IsValid() bool` 的枚举类型。

#### Scenario: Accept valid enum
- **Given** DTO 字段使用 `validate:"enum"` 且字段值实现 `IsValid() bool`
- **When** `IsValid()` 返回 true
- **Then** 系统必须判定该字段校验通过

#### Scenario: Reject invalid enum
- **Given** DTO 字段使用 `validate:"enum"` 且字段值实现 `IsValid() bool`
- **When** `IsValid()` 返回 false
- **Then** 系统必须判定该字段校验失败

#### Scenario: Reject misconfigured enum without panic
- **Given** DTO 字段使用 `validate:"enum"` 但字段值未实现枚举接口或为 nil 指针
- **When** 系统执行 enum 校验
- **Then** 系统必须判定该字段校验失败
- **Then** 系统不得 panic
