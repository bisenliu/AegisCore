## ADDED Requirements

### Requirement: Parse Bearer authorization headers consistently

系统 SHALL 在 `common/security/auth` 包提供统一 Bearer authorization header 解析函数，用于要求调用方必须提供 `Authorization: Bearer <token>` 语义的入口。该解析函数 MUST 先 trim header 首尾空白；Bearer 前缀 MUST 按大小写无关方式匹配；提取后的 token MUST trim 首尾空白；缺少 Bearer 前缀和 Bearer token 为空 MUST 可被调用方区分；token 原文除首尾空白外 MUST NOT 被修改。

#### Scenario: Parse bearer authorization header
- **Given** 调用方提供 Authorization header `Bearer abc.def.ghi`
- **When** 调用方使用 shared auth Bearer authorization 解析函数
- **Then** 系统 MUST 返回 token `abc.def.ghi`
- **Then** 系统 MUST 将该 header 识别为有效 Bearer authorization 格式

#### Scenario: Parse bearer prefix case-insensitively
- **Given** 调用方提供 Authorization header `bearer abc.def.ghi`
- **When** 调用方使用 shared auth Bearer authorization 解析函数
- **Then** 系统 MUST 返回 token `abc.def.ghi`
- **Then** 系统 MUST 将该 header 识别为有效 Bearer authorization 格式

#### Scenario: Reject authorization header without bearer prefix
- **Given** 调用方提供 Authorization header `abc.def.ghi`
- **When** 调用方使用 shared auth Bearer authorization 解析函数
- **Then** 系统 MUST 将该 header 识别为格式错误
- **Then** 系统 MUST NOT 返回有效 token

#### Scenario: Reject empty bearer authorization token
- **Given** 调用方提供 Authorization header `Bearer `
- **When** 调用方使用 shared auth Bearer authorization 解析函数
- **Then** 系统 MUST 将该 header 识别为空 Bearer token
- **Then** 系统 MUST NOT 返回有效 token

#### Scenario: Preserve token contents after trimming
- **Given** 调用方提供 Authorization header `  Bearer AbC.Def.GhI  `
- **When** 调用方使用 shared auth Bearer authorization 解析函数
- **Then** 系统 MUST 返回 token `AbC.Def.GhI`
- **Then** 系统 MUST NOT 修改 token 内部字符大小写

## MODIFIED Requirements

### Requirement: Provide authentication transport and context credentials

系统 SHALL 在 `common/security/auth` 包提供认证传输常量、Bearer authorization 解析 helper 和认证上下文 helper，用于表达 Authorization header、Bearer token 类型、Bearer 前缀、认证用户 ID 和认证会话 ID。常量值 MUST 与现有 HTTP 认证契约保持一致。系统 MUST 提供 `StripBearerPrefix(token string) string` 统一剥离可选 Bearer 前缀：该 helper MUST 先 trim token 首尾空白；当前缀按大小写无关匹配等于 `Bearer ` 时，MUST 返回剥离前缀后的 token；当前缀不存在时，MUST 返回 trim 后的原 token。系统 MUST 同时提供要求 Bearer 前缀存在的 Authorization header 解析入口，用于 HTTP 认证边界统一提取 token 并区分格式错误与空 token。

#### Scenario: Provide bearer authorization constants
- **When** 调用方读取 auth 认证传输常量
- **Then** `auth.AuthorizationHeader` MUST 等于 `Authorization`
- **Then** `auth.TokenTypeBearer` MUST 等于 `Bearer`
- **Then** `auth.TokenPrefix` MUST 等于 `Bearer `

#### Scenario: Strip bearer prefix from token
- **Given** 调用方提供 token 字符串 `Bearer abc.def.ghi`
- **When** 调用方使用 `auth.StripBearerPrefix`
- **Then** 系统 MUST 返回 `abc.def.ghi`

#### Scenario: Strip bearer prefix case-insensitively
- **Given** 调用方提供 token 字符串 `bearer abc.def.ghi`
- **When** 调用方使用 `auth.StripBearerPrefix`
- **Then** 系统 MUST 返回 `abc.def.ghi`

#### Scenario: Strip surrounding whitespace before bearer parsing
- **Given** 调用方提供 token 字符串 `  Bearer abc.def.ghi  `
- **When** 调用方使用 `auth.StripBearerPrefix`
- **Then** 系统 MUST 返回 `abc.def.ghi`

#### Scenario: Keep raw token without bearer prefix
- **Given** 调用方提供 token 字符串 `abc.def.ghi`
- **When** 调用方使用 `auth.StripBearerPrefix`
- **Then** 系统 MUST 返回 `abc.def.ghi`

#### Scenario: Store and read authenticated user id
- **Given** 调用方持有 `context.Context` 和认证用户 ID
- **When** 调用方使用 `auth.WithUserID` 写入用户 ID
- **Then** 调用方 MUST 能使用 `auth.UserIDFromContext` 读取相同用户 ID
- **Then** 空用户 ID 或缺失用户 ID MUST NOT 被读取为有效认证用户

#### Scenario: Store and read authenticated session id
- **Given** 调用方持有 `context.Context` 和认证会话 ID
- **When** 调用方使用 `auth.WithSessionID` 写入会话 ID
- **Then** 调用方 MUST 能使用 `auth.SessionIDFromContext` 读取相同会话 ID
- **Then** 空会话 ID 或缺失会话 ID MUST NOT 被读取为有效认证会话
