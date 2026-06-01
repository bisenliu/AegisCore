## ADDED Requirements

### Requirement: Keep validation implementation modular

系统 MUST 将共享请求校验能力组织为职责清晰的实现单元，避免单个文件同时承担 Fx module、validator 初始化、binding、反射字段绑定、错误归一化、字段名解析、翻译注册和响应日志集成。

#### Scenario: Maintain focused validation files
- **Given** 维护者需要修改 binding、错误归一化、字段名解析、翻译或 Fx module
- **When** 查看 `common/validation` 包
- **Then** 每类职责必须位于聚焦文件中
- **Then** 修改某一职责不得要求理解一个包含全部校验逻辑的聚合大文件

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
