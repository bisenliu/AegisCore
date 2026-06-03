## MODIFIED Requirements

### Requirement: Create user with validated JSON request
系统必须通过 `POST /api/v1/users` 创建用户，并要求调用方提供有效的 Bearer token。认证通过后，controller 必须使用 `common/validation` 绑定并校验 JSON 请求体，成功时必须返回统一成功响应信封和创建后的用户资料。认证失败时，系统必须在进入 controller 前返回统一未认证响应。

#### Scenario: Create user successfully
- **Given** 数据库中不存在用户名为 `alice` 的用户
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `POST /api/v1/users` 并提交包含 `name`、`username`、`password` 的合法 JSON 请求体
- **Then** controller 使用共享校验器绑定 JSON 请求体
- **Then** service 生成 UUIDv7 `user_id` 并创建用户记录
- **Then** service 使用 `common` 密码哈希方法将明文密码转换为 Argon2id hash 后持久化
- **Then** 系统返回 HTTP 201
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `0`，`message` 为 `created`
- **Then** `data` 包含新用户的 `user_id`、`name`、`username`、`active`、`created_at`、`updated_at`
- **Then** `data.user_id` 必须为 UUID 字符串
- **Then** `data.created_at` 和 `data.updated_at` 必须为毫秒级 Unix 时间戳
- **Then** `data` 不得包含 `id`、`email`、`password` 或密码哈希

#### Scenario: Reject unauthenticated create request
- **Given** 调用方未携带有效 Bearer token
- **When** 调用方请求 `POST /api/v1/users` 并提交合法 JSON 请求体
- **Then** 系统返回 HTTP 401
- **Then** 响应信封的 `success` 为 `false`
- **Then** 响应信封的 `code` 为 `20000`
- **Then** 请求不得进入 `UserController.Create`

#### Scenario: Reject empty JSON body
- **Given** 调用方没有提交请求体
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `POST /api/v1/users`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `success` 为 `false`
- **Then** 响应信封的 `message` 为共享校验器提供的中文公开错误消息

### Requirement: Validate create user input fields
系统必须对创建用户请求执行数据校验和业务校验，至少覆盖必填项、字段长度限制、枚举或默认值、用户名唯一性、用户已存在冲突和密码必填约束。

#### Scenario: Reject missing required fields
- **Given** 创建用户请求缺少 `name`、`username` 或 `password`
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `10001`
- **Then** 响应信封的 `message` 为中文化参数校验失败消息
- **Then** 响应不写入用户记录

#### Scenario: Reject fields exceeding length limits
- **Given** 创建用户请求的 `name` 超过 128 字符或 `username` 超过 255 字符
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `10001`
- **Then** 响应不写入用户记录

#### Scenario: Apply active default
- **Given** 创建用户请求没有显式提交 `active`
- **When** 系统创建用户
- **Then** 新用户的 `active` 必须为 `true`

#### Scenario: Reject invalid enum value when enum fields are introduced
- **Given** 创建用户请求包含枚举型字段且字段值不在允许范围内
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `10001`

### Requirement: Enforce unique user identity
系统必须以用户名作为创建用户的唯一业务身份。若用户名已存在，系统必须返回统一冲突错误，不得创建重复用户。

#### Scenario: Reject existing user before create
- **Given** 数据库中已存在用户名为 `alice` 的用户
- **When** 调用方请求创建相同用户名的用户
- **Then** service 必须识别用户已存在
- **Then** 系统返回 HTTP 409
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `40000`
- **Then** 响应信封的 `message` 使用 `user-services/internal/apperror` 中维护的用户已存在文案

#### Scenario: Convert database uniqueness violation to conflict
- **Given** 并发创建导致 `username` 或 `user_id` 唯一索引在数据库写入时冲突
- **When** repository 收到 Ent 唯一约束错误
- **Then** repository 或 service 必须将错误转换为冲突应用错误
- **Then** 系统返回 HTTP 409 和统一失败响应信封

### Requirement: Preserve create user persistence constraints
系统必须复用 Ent `User` schema 和 `users` 表作为创建用户的数据结构来源，只有在现有 schema 无法表达创建约束时才允许通过 Ent schema 和 Atlas migration 做最小变更。

#### Scenario: Create user through Ent repository
- **Given** 创建用户请求已通过校验和业务检查
- **When** repository 写入用户记录
- **Then** repository 使用具名 Ent client `user_db`
- **Then** repository 必须设置 `user_id`、`name`、`username`、`active` 和 Argon2id `password` hash
- **Then** repository 不得设置或持久化 `email`
- **Then** repository MUST 使用 `ExistsByUsername` 或等价 username 语义方法检查用户唯一性
- **Then** repository MUST NOT 保留 `ExistsByEmail` 方法、email 输入字段或 email 查询谓词
- **Then** `created_at` 和 `updated_at` 必须由 Ent schema 默认值写入毫秒级 Unix 时间戳
- **Then** 不得绕过 repository 直接在 controller 中访问数据库

#### Scenario: Preserve generated code workflow
- **Given** 实现创建用户时需要调整 Ent schema
- **When** 开发者完成 schema 修改
- **Then** 必须在 `user-services` 模块运行 `go generate ./ent`
- **Then** 必须通过 Atlas 脚本生成并校验 migration
- **Then** 不得手写 `user-services/ent/` 下的生成代码

### Requirement: Protect password from user profile responses
系统必须把 `password` 作为写入所需字段处理，不得在创建用户成功响应、查询用户响应、业务日志或错误消息中公开密码明文、盐、Argon2id 参数或完整密码哈希。

#### Scenario: Create response excludes password
- **Given** 创建用户请求包含合法 `password`
- **When** 系统成功创建用户并返回响应
- **Then** 响应 `data` 不得包含 `password` 或密码哈希
- **Then** 业务日志不得记录 `password` 的明文值或完整密码哈希

## ADDED Requirements

### Requirement: Generate stable external user id on creation
系统 SHALL 在创建用户时生成 UUIDv7 `user_id`，并将其作为用户对外身份。客户端不得在创建请求中指定 `user_id`，系统不得把内部数据库 `id` 作为用户资料 API 的对外身份。

#### Scenario: Created user receives UUIDv7 user id
- **Given** 调用方提交有效创建用户请求
- **When** 系统持久化新用户
- **Then** 系统 MUST 自动生成非空 UUIDv7 `user_id`
- **Then** PostgreSQL 用户记录的 `user_id` MUST 唯一且不可变
- **Then** 创建用户响应 MUST 返回 `user_id`
- **Then** 创建用户响应 MUST NOT 返回内部 `id`

#### Scenario: Client cannot set user id during creation
- **Given** 调用方在创建用户请求中携带 `user_id`
- **When** 系统处理创建用户请求
- **Then** 系统 MUST 忽略客户端提供的 `user_id` 或按请求校验规则拒绝该字段
- **Then** 新用户的 `user_id` MUST 使用服务端生成值
