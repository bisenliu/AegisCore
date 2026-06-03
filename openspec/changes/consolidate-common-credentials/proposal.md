## Why

当前 `common` 模块中的密码哈希、JWT 令牌和认证上下文工具分散在 `common/password`、`common/jwt` 与 `common/contextutil`，它们都属于凭证产生、校验、传输和上下文绑定的共享认证边界。将这些逻辑聚合到 `common/credentials` 可以降低调用方认知成本，并为后续 MFA、OAuth/OIDC、API Signature 等凭证类型保留一致扩展点。

## What Changes

- 新增 `common/credentials` 包，采用统一包名 `credentials` 的平铺文件组织：`password.go`、`jwt.go`、`context.go`。
- 将密码凭证能力迁移为 `credentials.HashPassword()` 与 `credentials.VerifyPassword()`，保留现有 Argon2id 参数、hash 格式和错误语义。
- 将 JWT 令牌凭证能力迁移为 `credentials.NewJWTService()`、`credentials.Claims`、`credentials.SignInput` 和访问/刷新/密码变更 token 签发与解析方法，保留现有签名、issuer、audience、subject、`user_id`、`token_version`、`session_id` 校验语义。
- 将认证传输与上下文绑定常量/函数迁移为 `credentials.AuthorizationHeader`、`credentials.TokenTypeBearer`、`credentials.TokenPrefix`、`credentials.UserIDKey`、`credentials.SessionIDKey`、`credentials.WithUserID()`、`credentials.UserIDFromContext()`、`credentials.WithSessionID()`、`credentials.SessionIDFromContext()`。
- 更新 common 与 user-services 内部调用方使用 `common/credentials`，避免继续在新代码中引用分散的凭证包。
- 保持 HTTP API、错误码、响应信封、JWT token 内容、密码 hash 格式和运行时配置兼容；本变更不引入新的认证协议或外部 API。

## Capabilities

### New Capabilities

- `common-credentials`: 统一管理 common 模块内密码、令牌、认证传输常量和认证上下文绑定等共享凭证原语。

### Modified Capabilities

- `user-authentication`: 将共享认证边界常量、JWT token 服务和认证上下文工具的规范来源调整为 `common/credentials`，但认证行为和失败响应保持不变。

## Impact

- 影响代码：`common/password/`、`common/jwt/`、`common/contextutil/auth.go`、`common/middleware/auth.go` 及其测试，可能还包括 user-services 中引用这些包的 auth/session 相关代码。
- API 兼容性：不改变 HTTP 路由、请求/响应格式、业务错误码或公开错误文案。
- 配置兼容性：继续使用现有 `config.AuthConfig` 与 JWT secret/issuer/audience 配置，不新增配置项。
- 数据兼容性：现有 Argon2id 密码 hash 和已签发 JWT claims 结构保持兼容。
- 依赖影响：不新增第三方依赖；继续使用 `github.com/golang-jwt/jwt/v5`、`golang.org/x/crypto/argon2` 和现有 Go workspace 模块边界。
