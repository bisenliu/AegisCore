# api-swagger-documentation

## Purpose

Swagger/OpenAPI 文档能力为用户服务提供可生成、可访问、与运行时路由和统一响应契约保持一致的 API 文档。

## Requirements

### Requirement: Provide Swagger metadata and generated OpenAPI artifacts
系统必须为用户服务提供 Swagger/OpenAPI 文档元数据、生成命令和生成产物，文档必须可由当前源码注解重复生成。

#### Scenario: Generate Swagger docs from user service entrypoint
- **Given** 用户服务源码包含全局 Swagger 注解和接口注解
- **When** 开发者在 `user-services` 模块执行记录的 Swagger 生成命令
- **Then** 系统生成或更新 `user-services/docs/docs.go`
- **Then** 系统生成或更新 `user-services/docs/swagger.json`
- **Then** 系统生成或更新 `user-services/docs/swagger.yaml`

#### Scenario: Swagger metadata describes current service
- **Given** Swagger 文档已生成
- **When** 调用方查看 OpenAPI 基础信息
- **Then** 文档包含 AegisCore 用户服务标题、版本、描述、host、BasePath、json consume/produce 信息
- **Then** 文档不得声明当前服务未实现的认证方案为接口必需项

### Requirement: Serve Swagger UI and document aliases
系统必须在用户服务运行时注册 Swagger 文档访问路由，并提供清晰的启用策略。

#### Scenario: Access Swagger UI in enabled environment
- **Given** HTTP server 已启动且 Swagger 已启用
- **When** 调用方请求 `GET /swagger/index.html`
- **Then** 系统返回 Swagger UI
- **Then** Swagger UI 能读取生成的 OpenAPI JSON

#### Scenario: Redirect docs aliases
- **Given** HTTP server 已启动且 Swagger 已启用
- **When** 调用方请求 `GET /docs` 或 `GET /api-docs`
- **Then** 系统重定向到 `/swagger/index.html`

#### Scenario: Disable Swagger in production by default
- **Given** 服务运行环境为生产环境且没有显式覆盖
- **When** 系统注册路由
- **Then** Swagger UI 和文档别名默认不对外注册

#### Scenario: Override Swagger enabled flag
- **Given** 环境变量 `SWAGGER_ENABLED` 被设置为 `true` 或 `false`
- **When** 系统注册路由
- **Then** 系统按环境变量显式值决定是否注册 Swagger 路由

### Requirement: Keep Swagger annotations aligned with runtime routes
系统必须确保 Swagger 注解中的 HTTP 方法、路径、请求参数、响应结构和错误状态码与真实 Gin 路由和响应契约一致。

#### Scenario: Document create user endpoint
- **Given** `POST /api/v1/users` 已注册
- **When** Swagger 文档生成
- **Then** 文档包含创建用户接口的标签、摘要、说明、JSON 请求体、成功响应和失败响应
- **Then** `@Router` 路径必须为 `/users [post]`
- **Then** 成功响应必须描述 HTTP 201 和 `common/response.Envelope` 包装的用户资料

#### Scenario: Document query user endpoint
- **Given** `GET /api/v1/users/:id` 已注册
- **When** Swagger 文档生成
- **Then** 文档包含路径参数 `id`、成功响应、400 参数错误、404 用户不存在和 500 内部错误
- **Then** `@Router` 路径必须为 `/users/{id} [get]`

#### Scenario: Document health endpoint according to actual response
- **Given** `GET /healthz` 返回最小健康状态 JSON
- **When** Swagger 文档生成
- **Then** 文档必须描述实际健康检查响应结构
- **Then** 文档不得把健康检查伪装为业务 API 响应信封

#### Scenario: Use unified error response examples
- **Given** Swagger 文档描述业务 API 的失败响应
- **When** 生成 OpenAPI schema
- **Then** 400、409、404 和 500 响应必须使用统一失败响应信封模型
- **Then** 响应说明必须包含调用方可理解的错误场景描述
