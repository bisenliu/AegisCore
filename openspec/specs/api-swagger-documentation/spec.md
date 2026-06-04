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
- **Then** 创建请求体必须描述 `nickname`、`username`、`password` 和可选 `status`
- **Then** `status` 必须描述允许值 `100`、`200`、`300`
- **Then** 成功响应必须描述 HTTP 201 和 `common/contract/response.Envelope` 包装的用户资料
- **Then** 成功响应用户资料必须包含 `user_id`、`nickname`、`username`、`status`、`created_at`、`updated_at`
- **Then** 成功响应不得描述 `name`、`active`、`password`、`password_hash` 或 `deleted_at`

#### Scenario: Document query user endpoint
- **Given** `GET /api/v1/users/:user_id` 已注册
- **When** Swagger 文档生成
- **Then** 文档包含路径参数 `user_id`、成功响应、400 参数错误、404 用户不存在和 500 内部错误
- **Then** `@Router` 路径必须为 `/users/{user_id} [get]`
- **Then** 成功响应用户资料必须包含 `user_id`、`nickname`、`username`、`status`、`created_at`、`updated_at`
- **Then** 文档必须说明查询默认不返回软删除用户

#### Scenario: Document user list endpoint
- **Given** `GET /api/v1/users` 已注册
- **When** Swagger 文档生成
- **Then** 文档包含分页参数 `page`、`page_size`
- **Then** 文档包含过滤参数 `nickname`、`username`、`status`
- **Then** 文档不包含旧过滤参数 `name` 或 `active`
- **Then** 文档描述 `status` 允许值 `100`、`200`、`300`
- **Then** 文档说明列表默认排除软删除用户

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

### Requirement: Document user schema field rename and soft delete contract
系统 MUST 在用户相关 Swagger/OpenAPI 文档中统一使用 `nickname`、`status` 和密码输入语义，且不得把内部持久化字段或软删除字段误暴露为普通响应字段。

#### Scenario: Generated schemas use new user field names
- **Given** Swagger 文档已生成
- **When** 调用方查看用户请求和响应 schema
- **Then** schema MUST 使用 `nickname` 表示用户昵称
- **Then** schema MUST 使用 `status` 表示用户状态
- **Then** schema MUST NOT 使用 `name` 或 `active` 表示用户资料字段

#### Scenario: Password hash is not documented as response data
- **Given** Swagger 文档已生成
- **When** 调用方查看创建、查询或列表用户响应 schema
- **Then** schema MUST NOT 包含 `password`
- **Then** schema MUST NOT 包含 `password_hash`
- **Then** 登录或创建请求中的密码字段说明 MUST 表示客户端提交的是密码输入而非数据库哈希字段

#### Scenario: Soft delete field remains internal
- **Given** Swagger 文档已生成
- **When** 调用方查看用户资料响应 schema
- **Then** schema MUST NOT 包含 `deleted_at`
- **Then** 接口说明 MUST 表达查询和列表默认只返回未软删除用户
