## MODIFIED Requirements

### Requirement: Query user by external user ID

系统必须通过 `GET /api/v1/users/:user_id` 接收外部用户 ID，并要求调用方提供有效的 Bearer token。认证通过后，系统使用共享请求校验能力校验 `user_id` 为 UUID 字符串，并返回对应未软删除用户资料。参数校验失败时，系统必须使用共享校验器提供的中文公开错误消息，不得由用户查询 controller 返回英文硬编码消息。认证失败时，系统必须在进入 `UserController.GetByUserID` 前返回统一未认证响应。repository MUST 将 Ent not found 转换为用户领域 `ErrUserNotFound`，service MUST 将该领域错误映射为现有 not found 应用错误。

#### Scenario: Query existing user
- **Given** 数据库中存在 `user_id` 为 `018f0000-0000-7000-8000-000000000001` 的用户，且包含 `nickname`、`username`、`status`、`password_hash`、`created_at`、`updated_at`、`deleted_at` 字段
- **Given** 该用户的 `deleted_at` 为 `NULL`
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/018f0000-0000-7000-8000-000000000001`
- **Then** 系统返回 HTTP 200
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `0`，`message` 为 `ok`
- **Then** `data` 包含用户的 `user_id`、`nickname`、`username`、`status`、`created_at`、`updated_at`
- **Then** `data.created_at` 和 `data.updated_at` 必须为毫秒级 Unix 时间戳
- **Then** `data` 不得包含 `password`、`password_hash`、`active`、`name` 或 `deleted_at`

#### Scenario: Reject unauthenticated query request
- **Given** 调用方未携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/018f0000-0000-7000-8000-000000000001`
- **Then** 系统返回 HTTP 401
- **Then** 响应信封的 `success` 为 `false`
- **Then** 响应信封的 `code` 为 `20000`
- **Then** 请求不得进入 `UserController.GetByUserID`

#### Scenario: Reject invalid user ID
- **Given** 调用方没有可用的 UUID 用户 ID
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/abc`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `10000`
- **Then** 响应信封的 `message` 为共享校验器提供的中文公开错误消息
- **Then** 响应信封的 `message` 不得为 `invalid user id`

#### Scenario: Reject blank user ID
- **Given** 调用方提供的用户 ID 为空或缺失
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/0`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `10000`
- **Then** 响应信封的 `message` 为共享校验器提供的中文公开错误消息
- **Then** 响应信封的 `message` 不得为 `invalid user id`

#### Scenario: User does not exist
- **Given** 数据库中不存在指定 `user_id` 且 `deleted_at` 为 `NULL` 的用户
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/018f0000-0000-7000-8000-000000000999`
- **Then** repository 将 Ent not found 转换为用户领域 `ErrUserNotFound`
- **Then** service 将 `ErrUserNotFound` 映射为 not found 应用错误
- **Then** 系统返回 HTTP 404
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `50000`，`message` 为 `用户不存在`

#### Scenario: Soft deleted user is not returned
- **Given** 数据库中存在指定 `user_id` 的用户
- **Given** 该用户的 `deleted_at` 不为 `NULL`
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/018f0000-0000-7000-8000-000000000001`
- **Then** repository 必须按未删除条件查询用户
- **Then** repository 将未匹配到未删除用户转换为用户领域 `ErrUserNotFound`
- **Then** 系统返回 HTTP 404
- **Then** 响应信封的 `message` 为 `用户不存在`
