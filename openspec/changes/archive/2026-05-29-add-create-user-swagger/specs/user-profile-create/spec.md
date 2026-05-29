## ADDED Requirements

### Requirement: Create user with validated JSON request
系统必须通过 `POST /api/v1/users` 创建用户，controller 必须使用 `common/validation` 绑定并校验 JSON 请求体，成功时必须返回统一成功响应信封和创建后的用户资料。

#### Scenario: Create user successfully
- **Given** 数据库中不存在邮箱为 `alice@example.com` 的用户
- **When** 调用方请求 `POST /api/v1/users` 并提交合法 JSON 请求体
- **Then** controller 使用共享校验器绑定 JSON 请求体
- **Then** service 创建用户记录
- **Then** 系统返回 HTTP 201
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `OK`，`message` 为 `created`
- **Then** `data` 包含新用户的 `id`、`name`、`email`、`active`、`created_at`、`updated_at`

#### Scenario: Reject empty JSON body
- **Given** 调用方没有提交请求体
- **When** 调用方请求 `POST /api/v1/users`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `success` 为 `false`
- **Then** 响应信封的 `message` 为共享校验器提供的中文公开错误消息

### Requirement: Validate create user input fields
系统必须对创建用户请求执行数据校验和业务校验，至少覆盖必填项、字段格式、长度限制、枚举或默认值、邮箱唯一性和用户已存在冲突。

#### Scenario: Reject missing required fields
- **Given** 创建用户请求缺少 `name` 或 `email`
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `VALIDATION_FAILED`
- **Then** 响应信封的 `message` 为中文化参数校验失败消息

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

### Requirement: Enforce unique user identity
系统必须以邮箱作为创建用户的唯一业务身份。若邮箱已存在，系统必须返回统一冲突错误，不得创建重复用户。

#### Scenario: Reject existing user before create
- **Given** 数据库中已存在邮箱为 `alice@example.com` 的用户
- **When** 调用方请求创建相同邮箱的用户
- **Then** service 必须识别用户已存在
- **Then** 系统返回 HTTP 409
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `CONFLICT`
- **Then** 响应信封的 `message` 使用 `user-services/internal/apperror` 中维护的用户已存在文案

#### Scenario: Convert database uniqueness violation to conflict
- **Given** 并发创建导致邮箱唯一索引在数据库写入时冲突
- **When** repository 收到 Ent 唯一约束错误
- **Then** repository 或 service 必须将错误转换为冲突应用错误
- **Then** 系统返回 HTTP 409 和统一失败响应信封

### Requirement: Preserve create user persistence constraints
系统必须复用 Ent `User` schema 和 `users` 表作为创建用户的数据结构来源，只有在现有 schema 无法表达创建约束时才允许通过 Ent schema 和 Atlas migration 做最小变更。

#### Scenario: Create user through Ent repository
- **Given** 创建用户请求已通过校验和业务检查
- **When** repository 写入用户记录
- **Then** repository 使用具名 Ent client `user_db`
- **Then** 不得绕过 repository 直接在 controller 中访问数据库

#### Scenario: Preserve generated code workflow
- **Given** 实现创建用户时需要调整 Ent schema
- **When** 开发者完成 schema 修改
- **Then** 必须在 `user-services` 模块运行 `go generate ./ent`
- **Then** 必须通过 Atlas 脚本生成并校验 migration
- **Then** 不得手写 `user-services/ent/` 下的生成代码
