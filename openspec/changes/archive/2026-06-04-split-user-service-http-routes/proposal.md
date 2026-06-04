## Why

当前 `user-services/internal/router/router.go` 中的 `RegisterRoutes` 同时注册 health、Swagger、认证、用户接口和认证中间件，函数名与职责边界偏宽泛。随着未来可能出现 admin、internal API、public API 或多版本 API，继续使用单一宽泛入口容易让用户服务 HTTP 路由注册点演变为难以维护的集中函数。

## What Changes

- 将用户服务 HTTP 路由总入口改为更明确的服务级命名，表达其覆盖用户服务完整 HTTP surface。
- 按 API surface 拆分路由注册逻辑：系统路由、Swagger 路由、v1 路由、公共认证路由、受保护认证路由、用户路由。
- 保持现有 HTTP 路径、方法、认证边界、Swagger 暴露规则和响应契约不变。
- 不新增 admin、internal API、v2 API 或授权能力，仅为后续扩展提供清晰挂载边界。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `http-service-runtime`: 明确用户服务 HTTP 路由注册入口应使用服务级命名，并按系统路由、文档路由、认证边界和业务资源路由分组组织。

## Impact

- 影响代码：`user-services/internal/router/router.go` 及新增的同包路由分组文件，`user-services/internal/bootstrap/routes.go` 的调用点。
- 影响测试：现有 bootstrap/router 相关测试应继续通过；如需要，可增加路由注册行为覆盖。
- API 兼容性：不改变现有 HTTP 路径、方法、状态码、认证要求或响应体格式。
- 依赖影响：不新增第三方依赖，不改变 Gin、Fx、Swagger 或认证中间件使用方式。
