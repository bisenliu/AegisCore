## MODIFIED Requirements

### Requirement: Normalize validation errors

系统必须把 binding、结构体校验和业务自定义校验错误归一化为调用方可读的校验错误，并允许 controller 使用 `common/response.Envelope` 输出失败响应。对于结构体 validator tag 校验失败，归一化错误必须包含可序列化的字段级明细。

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
- **Then** 响应必须使用 `common/response.Envelope`
- **Then** 响应必须为 HTTP 400 和 `BAD_REQUEST` 错误码，除非 controller 显式映射为已有兼容消息

#### Scenario: Include validation error details in envelope
- **Given** controller 使用共享校验器处理 validator tag 校验失败
- **When** controller 输出失败响应
- **Then** 响应必须为 HTTP 400 和参数校验失败业务码 `10001`
- **Then** 响应顶层 `message` 必须为 `请求参数验证失败`
- **Then** 响应 `errors` 数组中每条明细必须包含 `field`、`label`、`rule`、`message`
