## Why

`common/http/middleware.AuthWithTokenVersionValidator` 当前保留未使用的 `config.AuthConfig` 参数，调用方会误以为认证中间件会读取该配置参与 JWT 解析或 token version 校验。该参数实际只造成共享 helper API 噪音，应在继续扩散前收敛为真实依赖边界。

## What Changes

- **BREAKING** 移除 `AuthWithTokenVersionValidator` 的 `config.AuthConfig` 参数，使函数签名只保留 logger、JWT service 和可选 token version validator。
- 删除 user-service router 层对 `params.AuthConfig` 的中间件透传，并移除仅为该透传存在的 `RouteParams.AuthConfig` 字段。
- 更新 provider 适配和相关测试构造，继续通过 `auth.NewJWTService(config.AuthConfig)` 表达 JWT 配置消费入口。
- 保持 JWT 解析、认证失败响应、token version 校验、日志字段和受保护路由行为不变。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `shared-platform-primitives`: 收紧共享 JWT 认证 middleware 的导出 API，移除未参与行为的 `config.AuthConfig` 参数。

## Impact

- 影响代码：`common/http/middleware/auth.go`、`common/http/middleware/auth_test.go`、`user-service/internal/router/router.go`、`user-service/internal/router/router_registration_test.go`、`user-service/internal/providers/routes.go` 和相关 provider 测试。
- 影响 API：`common/http/middleware.AuthWithTokenVersionValidator` 的 Go 函数签名发生 breaking change；仓库内调用方需同步更新。
- 不影响 HTTP 外部契约、OpenAPI 输出、数据库 schema、Atlas migration、部署资产或观测指标。
- 不改变认证运行时行为；JWT 配置仍由 `auth.NewJWTService(config.AuthConfig)` 消费，token version 校验仍由 `auth.TokenVersionValidator` 控制。
