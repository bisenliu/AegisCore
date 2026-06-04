## ADDED Requirements

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
- **Then** 响应 MUST 继续使用 `common/response.Envelope`
- **Then** HTTP 状态码、业务错误码、公开 message 和字段级 `errors` 明细 MUST 与拆分前保持兼容

#### Scenario: Gin adapter logs and aborts failed requests
- **Given** controller 使用共享 Gin 校验适配层的失败出口
- **When** binding 或结构体校验失败
- **Then** 系统 MUST 通过共享 logger 记录 error 级别日志
- **Then** 日志 MUST 包含请求 path 和原始错误信息
- **Then** Gin 请求上下文 MUST 被 abort，避免 controller 继续执行业务逻辑
