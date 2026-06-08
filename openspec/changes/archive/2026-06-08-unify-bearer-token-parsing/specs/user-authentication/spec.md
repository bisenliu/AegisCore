## MODIFIED Requirements

### Requirement: Authenticate HTTP requests with JWT Bearer tokens

系统 MUST 提供可复用的 Gin 认证中间件，用于校验 `Authorization: Bearer <token>` 请求头。认证中间件 MUST 通过 `common/security/auth` 的 shared Bearer authorization 解析入口提取 token；Bearer 前缀匹配 MUST 大小写无关，token 原文除首尾空白外 MUST NOT 被修改。认证中间件 MUST 只保护实际挂载该中间件的路由，不得读取配置化白名单路径，也不得基于请求路径自行判断认证豁免；公开访问必须由服务侧通过不挂载认证中间件的路由分组表达。挂载认证中间件的请求缺少认证头、认证头格式错误、token 为空或 token 无效时，系统 MUST 返回 HTTP 401，并使用 `common/contract/response.Envelope` 失败格式与未认证数字业务码 `20000`。当 token 通过签名、过期时间、issuer 和 audience 校验后，中间件 MUST 解析 `user_id`、`token_version` 和 `session_id`，其中 `user_id` MUST 是用户外部 UUID 标识，并将 token 中的 `token_version` 与服务端当前版本比较；版本不一致时系统 MUST 将该 token 视为无效。

#### Scenario: Public route bypasses authentication by not mounting middleware
- **Given** 服务侧将公开路由注册到未挂载认证中间件的路由分组
- **When** 调用方请求该公开路由且未携带 `Authorization` header
- **Then** 认证中间件 MUST NOT 处理该请求
- **Then** 系统 MUST NOT 因缺少认证 header 返回未认证错误

#### Scenario: Reject missing authorization header
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方未携带 `Authorization` header
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应信封的 `success` MUST 为 `false`
- **Then** 响应信封的 `code` MUST 为 `20000`

#### Scenario: Reject invalid bearer format
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方携带不以大小写无关 `Bearer ` 前缀开头的 `Authorization` header
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应 MUST 使用统一失败响应信封

#### Scenario: Accept bearer prefix case-insensitively
- **Given** 请求命中已挂载认证中间件的路由
- **Given** 调用方携带 `Authorization: bearer <token>` header
- **Given** token 是签名有效、未过期且 claims 合法的 Access Token
- **When** 认证中间件校验该请求
- **Then** 系统 MUST 将该请求视为已认证
- **Then** 系统 MUST 继续执行后续 handler

#### Scenario: Reject empty bearer token
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方携带 `Authorization: Bearer `
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 系统 MUST NOT 调用后续业务 handler

#### Scenario: Reject invalid token
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方携带无法通过 JWT 解析、签名校验或标准 claims 校验的 token
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 认证失败日志 MUST NOT 记录 token 原文

#### Scenario: Reject mismatched token version
- **Given** 请求命中已挂载认证中间件的路由
- **Given** Access Token 签名有效且未过期
- **Given** Access Token claims 中的 `token_version` 与服务端当前版本不一致
- **When** 调用方携带该 Access Token 请求受保护 API
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 系统 MUST NOT 将该请求标记为已认证

### Requirement: Reuse shared authentication boundary constants

系统 SHALL 使用 `common/security/auth` 包中统一定义的认证边界常量和 Bearer authorization 解析入口表达 Authorization header、Bearer token type 和 Bearer authorization prefix，避免调用方重复硬编码这些协议值或重新实现 Bearer token 提取规则。

#### Scenario: Use shared authorization header constant
- **WHEN** 认证中间件读取请求认证信息
- **THEN** 系统 MUST 使用 `common/security/auth` 中的 Authorization header 常量
- **THEN** header 名值 MUST 保持为 `Authorization`

#### Scenario: Use shared bearer prefix constant
- **WHEN** 认证中间件校验或剥离 Authorization header
- **THEN** 系统 MUST 使用 `common/security/auth` 中的 Bearer prefix 常量或 Bearer authorization 解析入口
- **THEN** prefix 值 MUST 保持为 `Bearer `

#### Scenario: Use shared bearer authorization parser
- **WHEN** 认证中间件从 Authorization header 提取 token
- **THEN** 系统 MUST 使用 `common/security/auth` 中的 shared Bearer authorization 解析入口
- **THEN** 中间件 MUST NOT 自行使用独立的大小写敏感 prefix 判断实现 token 提取

#### Scenario: Use shared bearer token type constant
- **WHEN** 登录或刷新接口响应 token type
- **THEN** 系统 MUST 使用 `common/security/auth` 中的 Bearer token type 常量
- **THEN** token type 值 MUST 保持为 `Bearer`
