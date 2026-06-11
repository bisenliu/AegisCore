## MODIFIED Requirements

### Requirement: Keep Swagger annotations aligned with runtime routes
系统 MUST 确保 Swagger 注解中的 HTTP 方法、路径、请求参数、响应结构和错误状态码与真实 Gin 路由和响应契约一致。

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
- **Then** 文档包含 keyset 分页参数 `cursor`、`page_size`
- **Then** 文档 MUST NOT 包含旧分页参数 `page` 或 `offset`
- **Then** 文档包含过滤参数 `nickname`、`username`、`status`
- **Then** 文档不包含旧过滤参数 `name` 或 `active`
- **Then** 文档描述 `status` 允许值 `100`、`200`、`300`
- **Then** 文档说明列表默认排除软删除用户
- **Then** 文档描述成功响应 `data.pagination` 包含 `page_size`、`next_cursor`、`has_next`
- **Then** 文档 MUST NOT 描述 `total` 或 `total_pages` 分页响应字段

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
