## ADDED Requirements

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
- **WHEN** 用户服务需要用户资料请求清洗、UUID 解析、分页规范化或用户服务特定跨字段校验
- **THEN** 相关规则 MUST 保持在用户服务自己的 validation 边界内
- **THEN** 实现 MUST NOT 因 `common` 目录重组而把服务特定规则移动到 `common/validation`
