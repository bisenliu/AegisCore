## ADDED Requirements

### Requirement: Split user service bootstrap by runtime responsibility
系统 SHALL 将 `user-services/internal/bootstrap` 组合根按职责组织为聚焦源文件，至少区分 Fx app/module 装配、Gin engine 创建、认证 provider、路由注册和 HTTP server lifecycle。Fx app/module 装配文件 MUST 使用职责名称，例如 `app.go` 或 `fx.go`，拆分完成后 MUST NOT 保留 `bootstrap/bootstrap.go` 作为泛化聚合文件。拆分 MUST 保持服务启动命令、Fx provider/invoke 集合、Gin 中间件顺序、路由注册、认证中间件挂载和优雅关闭行为等价。

#### Scenario: Bootstrap file split preserves Fx app creation
- **Given** 调用方通过 `bootstrap.NewApp(configPath)` 创建用户服务 Fx app
- **When** bootstrap 代码按职责拆分为多个源文件
- **Then** `NewApp` MUST 继续提供 `commoninfra.ConfigPath(configPath)`、共享配置 provider 和 Zap logger provider
- **Then** `UserServiceModule` MUST 继续引入 timezone 与 validation module
- **Then** Fx app/module 装配 MUST 位于 `app.go` 或 `fx.go` 等职责命名文件中
- **Then** 实现 MUST NOT 继续使用 `bootstrap.go` 承载泛化 bootstrap 聚合职责
- **Then** 用户服务声明的 provider 和 invoke 行为 MUST 与拆分前等价

#### Scenario: Middleware and routes remain equivalent
- **Given** HTTP server 启动并注册 Gin engine
- **When** Gin engine 创建和路由注册逻辑移动到聚焦文件
- **Then** trace-id、recovery、request logger 和 CORS 中间件顺序 MUST 保持不变
- **Then** 健康检查、Swagger、用户 API 和认证 API 路由 MUST 保持当前注册语义
- **Then** 受保护路由的认证中间件挂载 MUST 保持不变

#### Scenario: HTTP server lifecycle remains equivalent
- **Given** 用户服务创建 HTTP server
- **When** HTTP server 创建和 lifecycle hook 移动到聚焦文件
- **Then** server MUST 继续监听配置中的 host 和 port
- **Then** read、write、idle 和 shutdown timeout MUST 继续使用加载后的 HTTP 配置和默认关闭超时规则
- **Then** `http.ErrServerClosed` MUST 继续不被记录为服务失败
