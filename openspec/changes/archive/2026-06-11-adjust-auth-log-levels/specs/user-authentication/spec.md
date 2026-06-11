## ADDED Requirements

### Requirement: Classify authentication failure logs by expectedness
系统 MUST 将认证中间件日志等级按失败来源分类。调用方缺少认证信息或提供不可接受凭证时，系统 MUST NOT 记录 Error 等级日志；真正表示服务端配置错误或依赖异常的认证失败路径 MUST 继续记录 Error 等级日志。认证失败日志 MUST NOT 记录 token 原文，并 MUST 保留请求 context 中的 `trace-id` 字段。

#### Scenario: Missing authorization header is not an error log
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方未携带 `Authorization` header
- **Then** 系统 MUST 返回现有 HTTP 401 未认证响应
- **Then** 认证中间件 MUST NOT 输出 Error 等级日志

#### Scenario: Malformed bearer credentials are warning logs
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方携带格式错误的 `Authorization` header 或空 Bearer token
- **Then** 系统 MUST 返回现有 HTTP 401 token invalid 响应
- **Then** 认证中间件 MUST 使用 Warn 等级记录该认证失败
- **Then** 认证失败日志 MUST NOT 包含 token 原文

#### Scenario: Invalid or expired access token is not an error log
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方携带无法通过 JWT 校验、subject 不匹配、claim 缺失或已过期的 Access Token
- **Then** 系统 MUST 返回现有 HTTP 401 认证失败响应
- **Then** 认证中间件 MUST NOT 输出 Error 等级日志
- **Then** 认证失败日志 MUST NOT 包含 token 原文

#### Scenario: Token version mismatch remains a warning log
- **Given** 请求命中已挂载认证中间件的路由
- **Given** Access Token 签名有效且未过期
- **Given** Access Token claims 中的 `token_version` 与服务端当前版本不一致
- **When** 认证中间件校验 token version
- **Then** 系统 MUST 返回现有 HTTP 401 token invalid 响应
- **Then** 认证中间件 MUST 使用 Warn 等级记录 token version mismatch
- **Then** 日志 MUST 包含结构化 `user_id` 和 token version 相关字段

#### Scenario: Authentication infrastructure failure remains an error log
- **Given** 请求命中已挂载认证中间件的路由
- **When** JWT secret 缺失或 token version validator 返回非版本不匹配的依赖异常
- **Then** 认证中间件 MUST 使用 Error 等级记录该服务端异常
- **Then** 系统 MUST 保持现有错误响应分类
