## MODIFIED Requirements

### Requirement: Support service-local validation boundaries
系统 MUST 允许 Gin controller 在共享请求校验通过后调用服务内校验器边界处理服务特定的输入清洗、基础校验、解析、转换和复杂跨字段规则。共享 `common/validation` MUST 保持跨服务通用能力定位，不得承载用户服务独有规则。用户服务本地校验器 MUST 位于 `user-services/internal/validators` 或等价服务内校验器边界，并可包含 `Normalize`、`Validate`、`Parse` 等函数。

#### Scenario: Controller delegates service-specific validation
- **Given** controller 已使用共享校验器完成请求绑定和结构体 tag 校验
- **When** 请求还需要用户服务特定的字符串清洗、分页规范化、UUID 解析、请求体 token 规范化或跨字段校验
- **Then** controller MUST 调用服务内 validators 层或等价校验器边界
- **Then** controller MUST NOT 将大量服务特定规则内联在 HTTP handler 中

#### Scenario: Shared validation remains generic
- **Given** 某个校验规则只适用于用户服务的用户资料请求
- **When** 开发者实现该规则
- **Then** 规则 MUST NOT 放入 `common/validation`
- **Then** 规则 MUST 位于用户服务自己的 validators 边界，除非该规则已成为多个服务稳定复用的通用能力

#### Scenario: Validation errors preserve envelope contract
- **Given** 服务内 validators 层判定请求级校验失败
- **When** controller 输出失败响应
- **Then** 响应 MUST 继续使用 `common/contract/response.Envelope`
- **Then** 错误状态码、业务码和公开消息 MUST 与现有请求校验错误语义兼容

#### Scenario: Validators support non-request inputs
- **Given** 用户服务需要在对象进入 Service 前校验配置对象、内部 input 或中间状态对象
- **When** 这些校验不依赖 Repository、Ent client、Redis client 或外部服务
- **Then** 相关规则 MUST 位于 `user-services/internal/validators` 或等价服务内校验器边界
- **Then** 包名 MUST NOT 将能力限制为 HTTP request 场景

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
