## ADDED Requirements

### Requirement: Reuse shared authentication boundary constants
系统 SHALL 使用 `common` 模块中统一定义的认证边界常量表达 Authorization header、Bearer token type 和 Bearer authorization prefix，避免调用方重复硬编码这些协议值。

#### Scenario: Use shared authorization header constant
- **WHEN** 认证中间件读取请求认证信息
- **THEN** 系统 MUST 使用 `common` 中的 Authorization header 常量
- **THEN** header 名值 MUST 保持为 `Authorization`

#### Scenario: Use shared bearer prefix constant
- **WHEN** 认证中间件校验或剥离 Authorization header
- **THEN** 系统 MUST 使用 `common` 中的 Bearer prefix 常量
- **THEN** prefix 值 MUST 保持为 `Bearer `

#### Scenario: Use shared bearer token type constant
- **WHEN** 登录或刷新接口响应 token type
- **THEN** 系统 MUST 使用 `common` 中的 Bearer token type 常量
- **THEN** token type 值 MUST 保持为 `Bearer`

### Requirement: Reuse shared authentication failure message
系统 SHALL 使用 `common` 模块中统一定义的认证失败公开文案返回缺失认证、token 非法、token 过期或 token version 不匹配等认证失败响应。

#### Scenario: Return shared authentication failure message
- **WHEN** 认证中间件拒绝缺失、格式错误、空值、非法、过期或版本不匹配的 token
- **THEN** 响应 message MUST 使用 `common` 中的统一认证失败公开文案
- **THEN** 公开文案值 MUST 保持为 `登录状态无效或已过期，请重新登录`

#### Scenario: Preserve authentication error classification
- **WHEN** 认证失败文案常量来源迁移到 `common`
- **THEN** 缺失认证信息 MUST 继续返回未认证业务码 `20000`
- **THEN** token 格式错误、非法或版本不匹配 MUST 继续返回 token invalid 业务码
- **THEN** token 过期 MUST 继续返回 token expired 业务码
- **THEN** 所有上述响应 MUST 继续使用 HTTP 401
