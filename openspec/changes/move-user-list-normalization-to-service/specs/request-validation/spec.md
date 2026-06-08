## MODIFIED Requirements

### Requirement: Support service-local validation boundaries
系统 MUST 允许服务在共享请求校验通过后通过服务内校验器边界处理服务特定的输入清洗、基础校验、解析、转换和复杂跨字段规则。共享 `common/validation` MUST 保持跨服务通用能力定位，不得承载用户服务独有规则。用户服务本地校验器 MUST 位于 `user-services/internal/validators` 或等价服务内校验器边界，并可包含 `Normalize`、`Validate`、`Parse` 等函数。Controller MUST 限定于请求绑定、结构性校验、调用 Service 前必须完成的简单解析步骤和响应输出。影响用例执行、需要 Repository/cache/外部资源或依赖当前业务状态的 Service 级业务归一化和校验 MUST 由 Service 编排，而不得由 Controller 或请求 DTO 方法执行。

#### Scenario: Controller delegates only transport-safe service-specific validation
- **Given** controller 已使用共享校验器完成请求绑定和结构体 tag 校验
- **When** 请求还需要用户服务特定的 UUID 解析或其他调用 Service 前必须完成的无资源依赖基础转换
- **Then** controller MUST 只调用不依赖外部资源且不改变用例执行语义的服务内 validators 层或等价校验器边界
- **Then** controller MUST NOT 将大量服务特定规则内联在 HTTP handler 中
- **Then** controller MUST NOT 调用会写入派生查询参数、改变用例执行语义或依赖外部资源的归一化/校验函数

#### Scenario: Service owns use-case normalization
- **Given** 用户服务需要分页规范化、过滤字段空白裁剪、请求体 token 规范化或会影响 Repository 输入的请求级清洗
- **When** Service 执行业务编排
- **Then** Service MUST 在访问 Repository 或外部资源前调用服务内 validators 层或等价校验器边界
- **Then** HTTP 和非 HTTP 调用路径 MUST 具有相同的用例输入归一化行为

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

#### Scenario: Service orchestrates resource-dependent business validation
- **Given** 创建或更新用例需要查询数据库、缓存、外部服务、当前时间窗口、资源锁定状态、价格一致性或业务实体状态
- **When** 系统校验该用例是否允许执行
- **Then** Service MUST 通过 repository、cache 或 domain 依赖编排校验
- **Then** 请求 DTO 的 `Validate` 方法和 Controller handler MUST NOT 为这些业务检查依赖 Gin context、全局数据库句柄、全局缓存客户端或全局 logger
