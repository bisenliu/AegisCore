## MODIFIED Requirements

### Requirement: Create user with validated JSON request
系统必须通过 `POST /api/v1/users` 创建用户，controller 必须使用 `common/validation` 绑定并校验 JSON 请求体，成功时必须返回统一成功响应信封和创建后的用户资料。

#### Scenario: Create user successfully
- **Given** 数据库中不存在邮箱为 `alice@example.com` 的用户
- **When** 调用方请求 `POST /api/v1/users` 并提交包含 `name`、`email`、`password` 的合法 JSON 请求体
- **Then** controller 使用共享校验器绑定 JSON 请求体
- **Then** service 创建用户记录并持久化 `password`
- **Then** 系统返回 HTTP 201
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `OK`，`message` 为 `created`
- **Then** `data` 包含新用户的 `id`、`name`、`email`、`active`、`created_at`、`updated_at`
- **Then** `data.created_at` 和 `data.updated_at` 必须为毫秒级 Unix 时间戳
- **Then** `data` 不得包含 `password`

#### Scenario: Reject empty JSON body
- **Given** 调用方没有提交请求体
- **When** 调用方请求 `POST /api/v1/users`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `success` 为 `false`
- **Then** 响应信封的 `message` 为共享校验器提供的中文公开错误消息

### Requirement: Validate create user input fields
系统必须对创建用户请求执行数据校验和业务校验，至少覆盖必填项、字段格式、长度限制、枚举或默认值、邮箱唯一性、用户已存在冲突和密码必填约束。

#### Scenario: Reject missing required fields
- **Given** 创建用户请求缺少 `name`、`email` 或 `password`
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `VALIDATION_FAILED`
- **Then** 响应信封的 `message` 为中文化参数校验失败消息
- **Then** 响应不写入用户记录

#### Scenario: Reject invalid email format
- **Given** 创建用户请求的 `email` 不是合法邮箱格式
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `VALIDATION_FAILED`
- **Then** 响应不写入用户记录

#### Scenario: Reject fields exceeding length limits
- **Given** 创建用户请求的 `name` 超过 128 字符或 `email` 超过 255 字符
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `VALIDATION_FAILED`
- **Then** 响应不写入用户记录

#### Scenario: Apply active default
- **Given** 创建用户请求没有显式提交 `active`
- **When** 系统创建用户
- **Then** 新用户的 `active` 必须为 `true`

#### Scenario: Reject invalid enum value when enum fields are introduced
- **Given** 创建用户请求包含枚举型字段且字段值不在允许范围内
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `VALIDATION_FAILED`

### Requirement: Preserve create user persistence constraints
系统必须复用 Ent `User` schema 和 `users` 表作为创建用户的数据结构来源，只有在现有 schema 无法表达创建约束时才允许通过 Ent schema 和 Atlas migration 做最小变更。

#### Scenario: Create user through Ent repository
- **Given** 创建用户请求已通过校验和业务检查
- **When** repository 写入用户记录
- **Then** repository 使用具名 Ent client `user_db`
- **Then** repository 必须设置 `name`、`email`、`active` 和 `password`
- **Then** `created_at` 和 `updated_at` 必须由 Ent schema 默认值写入毫秒级 Unix 时间戳
- **Then** 不得绕过 repository 直接在 controller 中访问数据库

#### Scenario: Preserve generated code workflow
- **Given** 实现创建用户时需要调整 Ent schema
- **When** 开发者完成 schema 修改
- **Then** 必须在 `user-services` 模块运行 `go generate ./ent`
- **Then** 必须通过 Atlas 脚本生成并校验 migration
- **Then** 不得手写 `user-services/ent/` 下的生成代码

## ADDED Requirements

### Requirement: Protect password from user profile responses
系统必须把 `password` 作为写入所需字段处理，不得在创建用户成功响应、查询用户响应、业务日志或错误消息中公开密码值。

#### Scenario: Create response excludes password
- **Given** 创建用户请求包含合法 `password`
- **When** 系统成功创建用户并返回响应
- **Then** 响应 `data` 不得包含 `password`
- **Then** 业务日志不得记录 `password` 的明文值
