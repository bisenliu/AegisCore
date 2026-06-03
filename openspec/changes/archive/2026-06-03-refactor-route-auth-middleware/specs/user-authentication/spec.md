## MODIFIED Requirements

### Requirement: Authenticate HTTP requests with JWT Bearer tokens

系统 MUST 提供可复用的 Gin 认证中间件，用于校验 `Authorization: Bearer <token>` 请求头。认证中间件 MUST 只保护实际挂载该中间件的路由，不得读取配置化白名单路径，也不得基于请求路径自行判断认证豁免；公开访问必须由服务侧通过不挂载认证中间件的路由分组表达。挂载认证中间件的请求缺少认证头、认证头格式错误、token 为空或 token 无效时，系统 MUST 返回 HTTP 401，并使用 `common/response.Envelope` 失败格式与未认证数字业务码 `20000`。当 token 通过签名、过期时间、issuer 和 audience 校验后，中间件 MUST 解析 `user_id`、`token_version` 和 `session_id`，其中 `user_id` MUST 是用户外部 UUID 标识，并将 token 中的 `token_version` 与服务端当前版本比较；版本不一致时系统 MUST 将该 token 视为无效。

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
- **When** 调用方携带不以 `Bearer ` 开头的 `Authorization` header
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应 MUST 使用统一失败响应信封

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

### Requirement: Log authentication decisions with caller-provided logger

系统 MUST 要求 Gin 认证中间件由调用方显式传入 Zap logger。认证中间件 MUST 使用该 logger 记录认证头缺失、认证头格式错误、空 bearer token、token 校验失败和 token version 校验失败等认证决策日志，并 MUST 继续通过请求 `context.Context` 保留 `trace-id` 日志字段。认证失败日志 MUST NOT 记录 token 原文。认证中间件 MUST NOT 输出白名单放行日志，因为公开访问由未挂载认证中间件的路由分组表达。

#### Scenario: Public route does not emit whitelist auth log
- **Given** 服务侧将公开路由注册到未挂载认证中间件的路由分组
- **When** 请求命中该公开路由
- **Then** 认证中间件 MUST NOT 输出白名单放行日志
- **Then** 该请求 MUST 继续执行对应公开 handler

#### Scenario: Log authentication failure with provided logger
- **Given** 调用方使用显式 Zap logger 构造认证中间件
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方未携带有效 `Authorization: Bearer <token>` 请求头
- **Then** 认证中间件 MUST 使用调用方传入的 logger 记录认证失败日志
- **Then** 认证失败日志 MUST NOT 包含 token 原文
- **Then** 系统 MUST 返回现有 HTTP 401 失败响应信封

#### Scenario: Preserve trace id in authentication logs
- **Given** 请求 context 中存在 trace id
- **Given** 调用方使用显式 Zap logger 构造认证中间件
- **When** 认证中间件输出认证相关日志
- **Then** 日志 MUST 包含请求 context 对应的 `trace-id` 字段
