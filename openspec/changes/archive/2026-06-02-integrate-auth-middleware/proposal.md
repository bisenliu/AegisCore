## Why

当前用户服务已具备统一响应、路由和共享中间件基础，但缺少认证能力，`/api/v1` 用户接口无法基于调用者身份进行访问控制。需要参考 `go-micro-scaffold/common/middleware/auth.go` 将 JWT Bearer 认证迁移到 AegisCore，并使用 `github.com/golang-jwt/jwt/v5` 版本实现，避免继续引入旧版 JWT 依赖。

## What Changes

- 新增基于 Bearer Token 的 Gin 认证中间件，支持白名单路径跳过认证、缺失/格式错误/无效 token 的统一未认证响应。
- 新增 JWT 解析能力和请求上下文身份字段，将认证后的 `user_id` 写入 Gin context 与 request context。
- 新增 auth 配置结构，支持通过 YAML 与 `AEGISCORE_` 环境变量配置 JWT secret、issuer、audience、过期策略和白名单路径。
- 调整用户服务运行时，在公开路径之外对 `/api/v1` 业务路由启用认证中间件。
- 使用 `github.com/golang-jwt/jwt/v5`，不迁移参考项目中的旧版 v4 JWT API；如参考代码存在 token 日志泄露风险，迁移时改为不记录 token 原文或截断内容。

## Capabilities

### New Capabilities

- `user-authentication`: 覆盖 JWT Bearer Token 认证中间件、认证配置、上下文身份传播和未认证错误响应。

### Modified Capabilities

- `http-service-runtime`: 用户服务运行时需要注册认证中间件，并区分公开路径与受保护业务 API。
- `shared-infrastructure`: 共享配置需要提供 auth/JWT 配置模型，供服务运行时装配认证依赖。
- `user-profile-query`: 用户资料查询接口从公开访问变为需要有效认证 token 才可访问。
- `user-profile-create`: 用户创建接口从公开访问变为需要有效认证 token 才可访问。

## Impact

- 代码影响：`common/config`、`common/middleware`、可能新增 `common/contextutil` 或等价上下文工具、可能新增 `common/jwt` 或等价 token 服务、`common/go.mod`、`user-services/internal/bootstrap`、`user-services/internal/router`、用户服务配置样例和相关测试。
- API 影响：受保护接口缺少 `Authorization: Bearer <token>`、Bearer 格式错误或 token 无效时返回 HTTP 401，响应仍使用 `common/response.Envelope`，错误码使用 `CodeUnauthenticated`。
- 配置影响：新增 `auth` 配置段；需要提供默认白名单覆盖 `/healthz`、Swagger 文档和其他明确公开路径。
- 依赖影响：新增 `github.com/golang-jwt/jwt/v5` 依赖；不引入旧版 v4 JWT 包。
- 数据模型影响：本变更不修改 Ent schema，不生成 Atlas migration。
