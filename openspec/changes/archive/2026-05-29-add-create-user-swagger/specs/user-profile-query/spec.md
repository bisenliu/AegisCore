## ADDED Requirements

### Requirement: Document query user API in Swagger
系统必须为现有 `GET /api/v1/users/:id` 查询用户接口提供与实际路由、请求参数和统一响应契约一致的 Swagger 注解和文档输出，不得改变查询接口运行时行为。

#### Scenario: Query endpoint appears in Swagger docs
- **Given** Swagger 文档已生成
- **When** 调用方查看用户接口分组
- **Then** 文档包含 `GET /users/{id}` 查询用户接口
- **Then** 文档包含 `id` 路径参数且说明其必须为正整数
- **Then** 文档描述 HTTP 200 成功响应为统一响应信封包装的用户资料

#### Scenario: Query endpoint documents failures
- **Given** Swagger 文档已生成
- **When** 调用方查看 `GET /users/{id}` 响应定义
- **Then** 文档包含 HTTP 400 参数错误响应
- **Then** 文档包含 HTTP 404 用户不存在响应
- **Then** 文档包含 HTTP 500 内部错误响应
