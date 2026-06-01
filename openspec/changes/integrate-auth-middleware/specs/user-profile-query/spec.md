## MODIFIED Requirements

### Requirement: Query user by positive ID

系统必须通过 `GET /api/v1/users/:id` 接收用户 ID，并要求调用方提供有效的 Bearer token。认证通过后，系统使用共享请求校验能力校验用户 ID 为正整数，并返回对应用户资料。参数校验失败时，系统必须使用共享校验器提供的中文公开错误消息，不得由用户查询 controller 返回英文硬编码消息。认证失败时，系统必须在进入 controller 前返回统一未认证响应。

#### Scenario: Query existing user
- **Given** 数据库中存在 ID 为 `123` 的用户，且包含 `name`、`email`、`active`、`created_at`、`updated_at` 字段
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/123`
- **Then** 系统返回 HTTP 200
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `OK`，`message` 为 `ok`
- **Then** `data` 包含用户的 `id`、`name`、`email`、`active`、`created_at`、`updated_at`

#### Scenario: Reject unauthenticated query request
- **Given** 调用方未携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/123`
- **Then** 系统返回 HTTP 401
- **Then** 响应信封的 `success` 为 `false`
- **Then** 响应信封的 `code` 为 `CodeUnauthenticated`
- **Then** 请求不得进入 `UserController.GetByID`

#### Scenario: Reject non-numeric user ID
- **Given** 调用方没有可用的数字用户 ID
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/abc`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `BAD_REQUEST`
- **Then** 响应信封的 `message` 为共享校验器提供的中文公开错误消息
- **Then** 响应信封的 `message` 不得为 `invalid user id`

#### Scenario: Reject non-positive user ID
- **Given** 调用方提供的用户 ID 小于或等于 `0`
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/0`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `BAD_REQUEST`
- **Then** 响应信封的 `message` 为共享校验器提供的中文公开错误消息
- **Then** 响应信封的 `message` 不得为 `invalid user id`

#### Scenario: User does not exist
- **Given** 数据库中不存在 ID 为 `999` 的用户
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users/999`
- **Then** repository 将 Ent not found 转换为 `user not found`
- **Then** 系统返回 HTTP 404
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `NOT_FOUND`，`message` 为 `user not found`
