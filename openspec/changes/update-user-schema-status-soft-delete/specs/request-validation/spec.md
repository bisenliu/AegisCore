## ADDED Requirements

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
