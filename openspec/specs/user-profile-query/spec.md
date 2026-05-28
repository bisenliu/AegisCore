# user-profile-query

## Purpose

用户资料查询能力允许 API 调用方通过用户 ID 获取用户基础资料，并把参数错误、用户不存在和内部查询错误转换为统一响应契约。

## Requirements

### Requirement: Query user by positive ID

系统必须通过 `GET /api/v1/users/:id` 接收用户 ID，校验其为正整数，并返回对应用户资料。

#### Scenario: Query existing user
- **Given** 数据库中存在 ID 为 `123` 的用户，且包含 `name`、`email`、`active`、`created_at`、`updated_at` 字段
- **When** 调用方请求 `GET /api/v1/users/123`
- **Then** 系统返回 HTTP 200
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `OK`，`message` 为 `ok`
- **Then** `data` 包含用户的 `id`、`name`、`email`、`active`、`created_at`、`updated_at`

#### Scenario: Reject non-numeric user ID
- **Given** 调用方没有可用的数字用户 ID
- **When** 调用方请求 `GET /api/v1/users/abc`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `BAD_REQUEST`，`message` 为 `invalid user id`

#### Scenario: Reject non-positive user ID
- **Given** 调用方提供的用户 ID 小于或等于 `0`
- **When** 调用方请求 `GET /api/v1/users/0`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `BAD_REQUEST`，`message` 为 `invalid user id`

#### Scenario: User does not exist
- **Given** 数据库中不存在 ID 为 `999` 的用户
- **When** 调用方请求 `GET /api/v1/users/999`
- **Then** repository 将 Ent not found 转换为 `user not found`
- **Then** 系统返回 HTTP 404
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `NOT_FOUND`，`message` 为 `user not found`

### Requirement: Preserve user model constraints

系统必须以 Ent `User` schema 作为用户资料查询的数据结构来源。

#### Scenario: User schema defines stable fields
- **Given** 用户资料由 Ent `User` schema 定义
- **When** 系统读取用户记录并映射响应 DTO
- **Then** 用户 ID 为唯一且不可变的 `int64`
- **Then** 用户 `email` 非空、唯一且最大长度为 255
- **Then** 用户 `name` 非空且最大长度为 128
- **Then** 响应包含 `active`、`created_at`、`updated_at`

#### Scenario: Repository returns unexpected database error
- **Given** 数据库查询用户时发生非 not found 错误
- **When** service 处理 repository 返回的错误
- **Then** 错误被转换为内部错误
- **Then** API 响应使用 `INTERNAL_ERROR` 和 `internal server error`，不暴露底层数据库细节
