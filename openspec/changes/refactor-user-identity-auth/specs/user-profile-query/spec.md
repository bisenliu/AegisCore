## MODIFIED Requirements

### Requirement: Query user by positive ID
系统必须通过 `GET /api/v1/users/:user_id` 接收用户外部 UUID 标识，并要求调用方提供有效的 Bearer token。认证通过后，系统使用共享请求校验能力校验 `user_id` 为合法 UUID 字符串，并返回对应用户资料。参数校验失败时，系统必须使用共享校验器提供的中文公开错误消息，不得由用户查询 controller 返回英文硬编码消息。认证失败时，系统必须在进入 controller 前返回统一未认证响应。

#### Scenario: Query existing user
- **Given** 数据库中存在 `user_id` 为 `018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e` 的用户，且包含 `name`、`username`、`active`、`password`、`created_at`、`updated_at` 字段
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e`
- **Then** 系统返回 HTTP 200
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `0`，`message` 为 `ok`
- **Then** `data` 包含用户的 `user_id`、`name`、`username`、`active`、`created_at`、`updated_at`
- **Then** `data.created_at` 和 `data.updated_at` 必须为毫秒级 Unix 时间戳
- **Then** `data` 不得包含 `id`、`email`、`password` 或密码哈希

#### Scenario: Reject unauthenticated query request
- **Given** 调用方未携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e`
- **Then** 系统返回 HTTP 401
- **Then** 响应信封的 `success` 为 `false`
- **Then** 响应信封的 `code` 为 `20000`
- **Then** 请求不得进入 `UserController.GetByID`

#### Scenario: Reject invalid user id format
- **Given** 调用方没有可用的 UUID 用户 ID
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/abc`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `10000`
- **Then** 响应信封的 `message` 为共享校验器提供的中文公开错误消息
- **Then** 响应信封的 `message` 不得为 `invalid user id`

#### Scenario: User does not exist
- **Given** 数据库中不存在 `user_id` 为 `018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e` 的用户
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e`
- **Then** repository 将 Ent not found 转换为 `user not found`
- **Then** 系统返回 HTTP 404
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `50000`，`message` 为 `user not found`

### Requirement: Preserve user model constraints
系统必须以 Ent `User` schema 作为用户资料查询的数据结构来源，并确保查询响应只公开用户资料字段，不公开内部主键或敏感凭据字段。

#### Scenario: User schema defines stable fields
- **Given** 用户资料由 Ent `User` schema 定义
- **When** 系统读取用户记录并映射响应 DTO
- **Then** 用户内部 `id` 为唯一且不可变的数据库主键
- **Then** 用户外部 `user_id` 为唯一、不可变且非空的 UUID 字段
- **Then** 用户 `username` 非空、唯一且最大长度为 255
- **Then** 用户 `name` 非空且最大长度为 128
- **Then** 用户 `password` 非空且不得映射到查询响应 DTO
- **Then** 用户 schema 不得继续定义 `email` 字段
- **Then** 响应包含 `active`、`created_at`、`updated_at`
- **Then** `created_at` 和 `updated_at` 必须为毫秒级 Unix 时间戳

#### Scenario: Repository returns unexpected database error
- **Given** 数据库查询用户时发生非 not found 错误
- **When** service 处理 repository 返回的错误
- **Then** 错误被转换为内部错误
- **Then** API 响应使用内部错误业务码 `90000` 和 `internal server error`，不暴露底层数据库细节

### Requirement: Document query user API in Swagger
系统必须为 `GET /api/v1/users/:user_id` 查询用户接口提供与实际路由、请求参数和统一响应契约一致的 Swagger 注解和文档输出，不得暴露内部数据库 `id` 或密码字段。

#### Scenario: Query endpoint appears in Swagger docs
- **Given** Swagger 文档已生成
- **When** 调用方查看用户接口分组
- **Then** 文档包含 `GET /users/{user_id}` 查询用户接口
- **Then** 文档包含 `user_id` 路径参数且说明其必须为 UUID 字符串
- **Then** 文档描述 HTTP 200 成功响应为统一响应信封包装的用户资料
- **Then** 文档描述 `created_at` 和 `updated_at` 为毫秒级 Unix 时间戳
- **Then** 文档不得把 `id`、`email` 或 `password` 描述为查询响应字段

#### Scenario: Query endpoint documents failures
- **Given** Swagger 文档已生成
- **When** 调用方查看 `GET /users/{user_id}` 响应定义
- **Then** 文档包含 HTTP 400 参数错误响应
- **Then** 文档包含 HTTP 404 用户不存在响应
- **Then** 文档包含 HTTP 500 内部错误响应
