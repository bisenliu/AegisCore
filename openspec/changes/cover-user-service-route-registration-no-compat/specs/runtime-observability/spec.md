## ADDED Requirements

### Requirement: 用户服务聚合运行时路由注册测试
系统 MUST 使用 router 包测试覆盖 user-service 聚合运行时路由注册结果，确保健康检查、OpenAPI、metrics 和 pprof 路由保持在当前路径和当前授权边界内。

#### Scenario: 健康检查与 OpenAPI 路由注册
- **WHEN** `RegisterUserServiceHTTPRoutes` 使用当前配置注册 HTTP 路由
- **THEN** 测试 MUST 验证 `/livez`、`/readyz`、`/startupz`、OpenAPI JSON 和 OpenAPI UI 或 redirect 路由注册在 `/api/v1` 外
- **AND** 测试 MUST 验证这些运行时路由不依赖业务认证或 RBAC 授权中间件

#### Scenario: metrics 配置错误返回
- **WHEN** metrics endpoint 配置为与健康检查、OpenAPI、`/api/v1` 或 pprof 保留前缀冲突的路径
- **THEN** `RegisterUserServiceHTTPRoutes` MUST 返回 metrics 配置错误
- **AND** 测试 MUST NOT 接受旧 metrics path 或旧兼容别名作为成功路径

#### Scenario: pprof 开关影响注册结果
- **WHEN** pprof 配置禁用
- **THEN** 测试 MUST 验证当前 pprof base path 未注册
- **AND** 当 pprof 配置启用时，测试 MUST 验证仅当前配置的 pprof base path 和 profile wildcard 被注册
