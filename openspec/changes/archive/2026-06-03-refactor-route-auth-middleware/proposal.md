## Why

当前认证豁免依赖 `auth.whitelist` 全局配置和前缀匹配，公开路由与受保护路由的安全边界隐藏在运行时配置中，容易因配置遗漏、误配或前缀碰撞导致访问控制行为不清晰。后续接入 Casbin 授权中间件时，需要先将“是否需要认证”从配置白名单迁移到路由局部挂载中间件，让公开、认证、授权分组在代码结构中可见且可测试。

## What Changes

- **BREAKING**: 从 `config.yaml` 和共享配置结构中移除 `auth.whitelist` 及其所有子项，不再支持通过配置白名单豁免认证。
- 修改共享认证中间件契约：认证中间件只负责校验挂载到它的路由请求，不再读取或判断白名单路径。
- 修改用户服务 HTTP 路由注册方式：全局中间件保留 trace-id、recovery、request logging 和 CORS；认证中间件改为挂载在受保护路由分组上。
- 建立面向 Casbin 的路由分组方案：公开路由、仅认证路由、认证加授权路由分层组织，为后续按资源和动作接入授权中间件预留清晰挂载点。
- 明确公开访问范围：健康检查、Swagger 文档、登录、刷新、首次强制改密入口保持公开或无需普通会话认证；用户资料 API、退出登录和退出全部设备等依赖当前会话的接口必须挂载认证中间件。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 认证配置加载契约移除 `auth.whitelist`，仅保留 JWT、token version cache TTL 和 refresh token rotation 等认证运行时配置。
- `user-authentication`: 认证中间件不再支持配置化白名单；认证豁免由服务路由分组是否挂载该中间件决定。
- `http-service-runtime`: 用户服务路由注册改为通过公开分组、认证分组和预留授权分组表达访问控制边界，而不是依赖全局认证中间件和白名单配置。

## Impact

- 受影响代码：`common/config/config.go`、`common/config/loader_test.go`、`common/middleware/auth.go`、`common/middleware/auth_test.go`、`user-services/configs/config.yaml`、`user-services/internal/bootstrap/bootstrap.go`、`user-services/internal/router/router.go` 及相关路由测试。
- 配置影响：部署配置中存在的 `auth.whitelist` 将不再生效；公开路由必须由代码中的公开路由分组注册。
- API 影响：路由路径和响应信封保持不变；缺少或无效认证的受保护路由仍返回 HTTP 401 和现有认证失败响应。
- 安全影响：减少通过配置扩大公开访问面的风险；为 Casbin 授权中间件后续按路由分组或资源动作局部挂载提供结构基础。
