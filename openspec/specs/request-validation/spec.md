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

系统必须把 binding、结构体校验和业务自定义校验错误归一化为调用方可读的校验错误，并允许 controller 使用 `common/contract/response.Envelope` 输出失败响应。对于结构体 validator tag 校验失败，归一化错误必须包含可序列化的字段级明细。

#### Scenario: Reject invalid field value
- **Given** 请求 DTO 字段包含 `validate:"required"`、`validate:"gt=0"` 或其他 validator tag
- **When** 请求字段不满足校验规则
- **Then** 系统必须返回可识别的校验错误
- **Then** 错误必须包含顶层 message `请求参数验证失败`
- **Then** 错误必须包含字段级明细，且每条明细包含请求字段名、字段显示名、触发规则和中文错误消息

#### Scenario: Reject JSON type mismatch
- **Given** 请求 DTO 字段期望整数、布尔、字符串、数组或对象类型
- **When** JSON body 中对应字段类型不匹配
- **Then** 系统必须返回可读的字段类型错误
- **Then** 错误消息不得暴露内部反射或 decoder 细节

#### Scenario: Preserve response envelope
- **Given** controller 使用共享校验器处理请求校验失败
- **When** controller 输出失败响应
- **Then** 响应必须使用 `common/contract/response.Envelope`
- **Then** 响应必须为 HTTP 400 和通用请求错误业务码 `10000`，除非 controller 显式映射为已有兼容消息

#### Scenario: Include validation error details in envelope
- **Given** controller 使用共享校验器处理 validator tag 校验失败
- **When** controller 输出失败响应
- **Then** 响应必须为 HTTP 400 和参数校验失败业务码 `10001`
- **Then** 响应顶层 `message` 必须为 `请求参数验证失败`
- **Then** 响应 `errors` 数组中每条明细必须包含 `field`、`label`、`rule`、`message`

### Requirement: Log validation failures with field details

系统必须在共享请求校验器的失败出口记录结构化错误日志。使用 `BindOrAbort` 处理请求校验失败时，日志必须使用 error 级别；当归一化校验错误包含字段级明细时，日志必须包含 `errors` 字段，且该字段必须复用对外响应中的字段级校验明细。

#### Scenario: Log validation failure at error level
- **Given** controller 使用共享校验器 `BindOrAbort` 绑定并校验请求
- **When** 请求绑定或校验失败
- **Then** 系统必须通过共享 logger 记录一条 error 级别日志
- **Then** 日志必须包含原始错误信息和请求 path

#### Scenario: Include field details in validation failure log
- **Given** controller 使用共享校验器 `BindOrAbort` 处理 validator tag 校验失败
- **When** 归一化校验错误包含字段级明细
- **Then** 系统记录的 error 日志必须包含 `errors` 字段
- **Then** `errors` 字段中的每条明细必须包含请求字段名、字段显示名、触发规则和中文错误消息

#### Scenario: Omit field details when unavailable
- **Given** controller 使用共享校验器 `BindOrAbort` 处理请求失败
- **When** 归一化错误不包含字段级明细
- **Then** 系统仍必须记录 error 级别日志
- **Then** 日志不得要求存在非空 `errors` 字段

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

### Requirement: Keep validation implementation modular

系统 MUST 将共享请求校验能力组织为职责清晰的实现单元，避免单个文件同时承担 Fx module、validator 初始化、binding、反射字段绑定、错误归一化、字段名解析、翻译注册和响应日志集成。

#### Scenario: Maintain focused validation files
- **Given** 维护者需要修改 binding、错误归一化、字段名解析、翻译或 Fx module
- **When** 查看 `common/validation` 包
- **Then** 每类职责必须位于聚焦文件中
- **Then** 修改某一职责不得要求理解一个包含全部校验逻辑的聚合大文件

### Requirement: Separate validation core from Gin adapter

系统 MUST 将共享请求校验能力拆分为纯校验核心和 Gin HTTP 适配层。纯校验核心 MUST 负责 validator 初始化、结构体校验、字段名解析、错误归一化、自定义 rule 和 DTO 扩展钩子；Gin HTTP 适配层 MUST 负责从 `gin.Context` 绑定 URI、query、JSON 和 form 请求参数，并处理失败响应、日志记录和 abort 控制流。

#### Scenario: Core validation has no Gin dependency
- **Given** 开发者需要在非 HTTP 或非 Gin 场景中复用共享结构体校验能力
- **When** 代码导入 `common/validation`
- **Then** 该核心包 MUST NOT 要求调用方使用 `gin.Context`
- **Then** 该核心包 MUST NOT 负责写入 HTTP 响应或调用 Gin abort 控制流

#### Scenario: Gin adapter preserves request binding behavior
- **Given** controller 使用共享 Gin 校验适配层绑定 URI、query、JSON 或 form 请求参数
- **When** 请求参数满足 DTO 绑定和校验规则
- **Then** 系统 MUST 将请求参数写入 DTO
- **Then** 系统 MUST 使用共享 `common/validation` validator 执行结构体校验和 DTO 扩展钩子

#### Scenario: Gin adapter preserves validation failure responses
- **Given** controller 使用共享 Gin 校验适配层处理请求校验失败
- **When** binding 或 validator tag 校验失败
- **Then** 响应 MUST 继续使用 `common/contract/response.Envelope`
- **Then** HTTP 状态码、业务错误码、公开 message 和字段级 `errors` 明细 MUST 与拆分前保持兼容

#### Scenario: Gin adapter logs and aborts failed requests
- **Given** controller 使用共享 Gin 校验适配层的失败出口
- **When** binding 或结构体校验失败
- **Then** 系统 MUST 通过共享 logger 记录 error 级别日志
- **Then** 日志 MUST 包含请求 path 和原始错误信息
- **Then** Gin 请求上下文 MUST 被 abort，避免 controller 继续执行业务逻辑

### Requirement: Reject trailing JSON values during binding

系统 MUST 在 JSON body binding 时拒绝首个 JSON 值之后的额外 JSON token 或额外 JSON 值，避免请求体被部分解析后仍被接受。

#### Scenario: Reject trailing JSON body
- **Given** controller 使用共享 JSON binder 绑定请求体
- **When** 请求体为 `{"id":1} {"id":2}` 或其他首个 JSON 值之后仍有非空内容的 body
- **Then** 系统必须将请求判定为 binding 或 validation 失败
- **Then** controller 必须能够返回 HTTP 400 失败信封

### Requirement: Support configurable strict JSON fields

系统 MUST 允许服务选择是否拒绝 JSON body 中 DTO 未声明的字段。默认行为必须保持兼容，严格 unknown-field 拒绝必须通过选项显式启用。

#### Scenario: Preserve default unknown field compatibility
- **Given** 服务未启用严格 JSON 字段选项
- **When** JSON body 包含 DTO 未声明字段
- **Then** 系统必须保持当前兼容 binding 行为

#### Scenario: Reject unknown field when strict mode enabled
- **Given** 服务启用严格 JSON 字段选项
- **When** JSON body 包含 DTO 未声明字段
- **Then** 系统必须将请求判定为 binding 或 validation 失败
- **Then** 失败消息不得暴露内部反射或 decoder 细节

### Requirement: Bind common custom field types safely

系统 MUST 扩展 URI、query 和 form 反射 binding 的字段类型支持，并在不支持的字段类型上返回可读错误而不是 panic。

#### Scenario: Bind text unmarshaler field
- **Given** 请求 DTO 字段实现 `encoding.TextUnmarshaler`
- **When** 共享 binder 从 URI、query 或 form 中读取该字段值
- **Then** 系统必须调用字段的文本反序列化逻辑
- **Then** 反序列化失败时必须返回可读 binding 错误

#### Scenario: Bind duration field
- **Given** 请求 DTO 字段类型为 `time.Duration`
- **When** 共享 binder 从 URI、query 或 form 中读取该字段值
- **Then** 系统必须按 Go duration 字符串或明确文档化的格式解析该值
- **Then** 解析失败时必须返回可读 binding 错误

#### Scenario: Bind embedded pointer struct fields
- **Given** 请求 DTO 包含匿名嵌入的结构体指针字段
- **When** 共享 binder 绑定 URI、query 或 form 参数
- **Then** 系统必须能够递归绑定嵌入结构体中的可设置字段
- **Then** nil 嵌入指针需要写入字段时必须被安全初始化

### Requirement: Centralize validation tags and rules

系统 MUST 在 `common/validation` 中集中维护请求字段 tag 名称、自定义 rule 名称和校验失败默认消息，避免在 binder、字段名解析和翻译逻辑中重复硬编码。

#### Scenario: Resolve request field tags consistently
- **Given** DTO 字段包含 `json`、`form`、`uri` 或 `query` tag
- **When** 系统解析对外字段名或绑定请求参数
- **Then** 系统必须使用同一组集中维护的请求 tag 名称

#### Scenario: Register enum rule consistently
- **Given** validator 初始化自定义 `enum` 规则
- **When** 系统注册校验函数和翻译消息
- **Then** 系统必须使用同一个集中维护的 rule 名称

### Requirement: Validate user status request parameters through shared enum rule
系统 MUST 要求所有用户 `status` 请求参数使用实现 `IsValid() bool` 的枚举类型，并通过 `validate:"enum"` 触发 `common/validation` 中注册的 `validateEnum`。用户 controller 和 service MUST NOT 重复实现状态取值列表校验。

#### Scenario: Validate status in JSON body
- **Given** 创建或更新用户请求 DTO 包含 JSON 字段 `status`
- **When** controller 使用共享校验器绑定并校验请求体
- **Then** DTO 字段类型 MUST 实现 `IsValid() bool`
- **Then** DTO 字段 tag MUST 包含 `validate:"enum"` 或包含该规则的组合校验
- **Then** 共享 `validateEnum` MUST 校验 `status` 是否为允许值

#### Scenario: Validate status in query parameters
- **Given** 用户列表或其他查询请求 DTO 包含 query 字段 `status`
- **When** controller 使用共享校验器绑定并校验 query 参数
- **Then** DTO 字段类型 MUST 实现 `IsValid() bool`
- **Then** DTO 字段 tag MUST 包含 `validate:"enum"` 或包含该规则的组合校验
- **Then** 共享 `validateEnum` MUST 校验 `status` 是否为允许值

#### Scenario: Reject invalid user status consistently
- **Given** 请求中的 `status` 不是 `100`、`200` 或 `300`
- **When** 共享校验器执行结构体校验
- **Then** 系统 MUST 返回参数校验失败
- **Then** 响应 MUST 使用统一失败信封和共享校验错误明细
- **Then** controller 和 service MUST NOT 返回自定义硬编码 status 错误消息

#### Scenario: Accept omitted optional status
- **Given** 请求 DTO 中的 `status` 是可选字段
- **Given** 调用方未提供 `status`
- **When** 共享校验器执行结构体校验
- **Then** DTO 默认值逻辑或空值表达 MUST 在 `validateEnum` 前保持可区分
- **Then** 系统 MUST 允许服务端默认值处理该字段

### Requirement: Support service-local validation boundaries
系统 MUST 允许服务在共享请求校验通过后通过 feature-local HTTP validation 边界处理服务特定的输入清洗、基础校验、解析、转换和复杂跨字段规则。共享 `common/validation` MUST 保持跨服务通用能力定位，不得承载用户服务独有规则。用户服务本地 HTTP 校验器 MUST 位于对应 feature 的 `transport/http/validation.go` 或同 package 等价文件，并可包含 `Normalize`、`Validate`、`Parse` 等函数。Controller MUST 限定于请求绑定、结构性校验、调用 Service 前必须完成的简单解析步骤、DTO 到 command/query 映射和响应输出。影响用例执行、需要 Repository/cache/外部资源或依赖当前业务状态的 Service 级业务归一化和校验 MUST 由 Service 编排，而不得由 Controller、HTTP validation 或请求 DTO 方法执行。

#### Scenario: Controller delegates only transport-safe service-specific validation
- **Given** controller 已使用共享校验器完成请求绑定和结构体 tag 校验
- **When** 请求还需要用户服务特定的 UUID 解析或其他调用 Service 前必须完成的无资源依赖基础转换
- **Then** controller MUST 只调用不依赖外部资源且不改变用例执行语义的 feature-local HTTP validation 边界
- **Then** controller MUST NOT 将大量服务特定规则内联在 HTTP handler 中
- **Then** controller MUST NOT 调用会写入派生查询参数、改变用例执行语义或依赖外部资源的归一化/校验函数

#### Scenario: Service owns use-case normalization
- **Given** 用户服务需要分页规范化、过滤字段空白裁剪、请求体 token 规范化或会影响 Repository 输入的请求级清洗
- **When** Service 执行业务编排
- **Then** Service MUST 在访问 Repository 或外部资源前调用 app service 内部逻辑、domain 规则或明确的纯函数边界完成用例输入归一化
- **Then** HTTP 和非 HTTP 调用路径 MUST 具有相同的用例输入归一化行为

#### Scenario: Shared validation remains generic
- **Given** 某个校验规则只适用于用户服务的用户资料请求或认证会话请求
- **When** 开发者实现该规则
- **Then** 规则 MUST NOT 放入 `common/validation`
- **Then** 规则 MUST 位于对应 feature 的 `transport/http` validation 边界，除非该规则已成为多个服务稳定复用的通用能力

#### Scenario: Validation errors preserve envelope contract
- **Given** feature-local HTTP validation 判定请求级校验失败
- **When** controller 输出失败响应
- **Then** 响应 MUST 继续使用 `common/contract/response.Envelope`
- **Then** 错误状态码、业务码和公开消息 MUST 与现有请求校验错误语义兼容

#### Scenario: Reject global validators for user and auth HTTP DTOs
- **Given** 开发者新增或修改用户资料或认证会话 HTTP request DTO 的清洗、基础解析或跨字段校验
- **When** 规则只属于单一 feature 的 HTTP transport 边界
- **Then** 规则 MUST 位于 `features/<feature>/transport/http`
- **Then** 实现 MUST NOT 新增或继续依赖 `user-services/internal/validators` 承载该规则

#### Scenario: Service orchestrates resource-dependent business validation
- **Given** 创建或更新用例需要查询数据库、缓存、外部服务、当前时间窗口、资源锁定状态、价格一致性或业务实体状态
- **When** 系统校验该用例是否允许执行
- **Then** Service MUST 通过 repository、cache、domain 或 app port 依赖编排校验
- **Then** 请求 DTO 的 `Validate` 方法和 Controller handler MUST NOT 为这些业务检查依赖 Gin context、全局数据库句柄、全局缓存客户端或全局 logger

### Requirement: Keep validation core separate from categorized Gin adapter path
请求校验能力 SHALL 保持通用校验核心与 Gin HTTP 适配层分离。通用 validator 初始化、结构体校验、字段名解析、错误归一化、自定义 rule 和 DTO 扩展钩子 MUST 保持在 `common/validation`；Gin binding、失败响应、日志记录和 abort 控制流 MUST 位于 `common/http/ginvalidation`。目录迁移 MUST 保持请求绑定、校验规则、错误明细和失败响应行为不变。

#### Scenario: Core validation remains Gin-independent
- **WHEN** 非 HTTP 或非 Gin 调用方导入 `common/validation`
- **THEN** 该包 MUST NOT 要求调用方使用 `gin.Context`
- **THEN** 该包 MUST NOT 写入 HTTP 响应或调用 Gin abort 控制流

#### Scenario: Gin adapter path changes without response changes
- **WHEN** controller 使用 `common/http/ginvalidation` 绑定 URI、query、JSON 或 form 请求参数
- **THEN** 请求参数绑定和结构体校验行为 MUST 与迁移前保持一致
- **THEN** 校验失败响应 MUST 继续使用统一失败信封和字段级 `errors` 明细

#### Scenario: Service-specific validation remains service-owned
- **WHEN** 用户服务需要用户资料请求清洗、UUID 解析、分页规范化、请求体 token 规范化或用户服务特定跨字段校验
- **THEN** 相关规则 MUST 保持在用户服务自己的 validators 边界内
- **THEN** 实现 MUST NOT 因服务内目录重命名而把服务特定规则移动到 `common/validation`

