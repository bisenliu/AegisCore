# user-profile-create

## Purpose

用户资料创建能力允许 API 调用方创建用户基础资料，并将请求校验、业务校验、唯一性冲突和持久化错误转换为统一响应契约。
## Requirements
### Requirement: Create user with validated JSON request
系统必须通过 `POST /api/v1/users` 创建用户，并要求调用方提供有效的 Bearer token。认证通过后，controller 必须使用 `common/validation` 绑定并校验 JSON 请求体，成功时必须返回统一成功响应信封和创建后的用户资料。认证失败时，系统必须在进入 controller 前返回统一未认证响应。

#### Scenario: Create user successfully
- **Given** 数据库中不存在用户名为 `alice` 且 `deleted_at` 为 `NULL` 的用户
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `POST /api/v1/users` 并提交包含 `nickname`、`username`、`password` 的合法 JSON 请求体
- **Then** controller 使用共享校验器绑定 JSON 请求体
- **Then** service 必须将请求密码转换为 `password_hash` 后创建用户记录
- **Then** 新用户的 `status` 必须为 `100`
- **Then** 新用户的 `deleted_at` 必须为 `NULL`
- **Then** 系统返回 HTTP 201
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `0`，`message` 为 `created`
- **Then** `data` 包含新用户的 `user_id`、`nickname`、`username`、`status`、`created_at`、`updated_at`
- **Then** `data.created_at` 和 `data.updated_at` 必须为毫秒级 Unix 时间戳
- **Then** `data` 不得包含 `password`、`password_hash`、`name`、`active` 或 `deleted_at`

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
系统必须对创建用户请求执行数据校验和业务校验，至少覆盖必填项、字段格式、长度限制、状态枚举或默认值、用户名唯一性、用户已存在冲突和密码必填约束。

#### Scenario: Reject missing required fields
- **Given** 创建用户请求缺少 `nickname`、`username` 或 `password`
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `10001`
- **Then** 响应信封的 `message` 为中文化参数校验失败消息
- **Then** 响应不写入用户记录

#### Scenario: Reject invalid username format
- **Given** 创建用户请求的 `username` 不符合用户名格式约束
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `10001`
- **Then** 响应不写入用户记录

#### Scenario: Reject fields exceeding length limits
- **Given** 创建用户请求的 `nickname` 超过 128 字符或 `username` 超过 255 字符
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `10001`
- **Then** 响应不写入用户记录

#### Scenario: Apply normal status default
- **Given** 创建用户请求没有显式提交 `status`
- **When** 系统创建用户
- **Then** 新用户的 `status` 必须为 `100`

#### Scenario: Reject invalid status enum value
- **Given** 创建用户请求包含 `status` 且字段值不是 `100`、`200` 或 `300`
- **When** controller 使用共享校验器校验请求 DTO
- **Then** 请求 DTO 的 `status` 字段必须通过 `validate:"enum"` 触发共享 `validateEnum`
- **Then** 系统返回 HTTP 400
- **Then** 响应信封的 `code` 为 `10001`
- **Then** 响应不写入用户记录

### Requirement: Enforce unique user identity
系统必须以用户名作为创建用户的唯一业务身份。若用户名已存在，系统必须返回统一冲突错误，不得创建重复用户。repository MUST 将创建时发生的 Ent 唯一约束错误转换为用户领域 `ErrUserAlreadyExists`，service MUST 将该领域错误映射为现有 conflict 应用错误。

#### Scenario: Reject existing user before create
- **Given** 数据库中已存在用户名为 `alice` 的用户
- **When** 调用方请求创建相同用户名的用户
- **Then** service 必须识别用户已存在
- **Then** 系统返回 HTTP 409
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `40000`
- **Then** 响应信封的 `message` 使用 `user-services/internal/errmsg` 中维护的用户已存在文案

#### Scenario: Convert database uniqueness violation to conflict
- **Given** 并发创建导致 `username` 或 `user_id` 唯一索引在数据库写入时冲突
- **When** repository 收到 Ent 唯一约束错误
- **Then** repository 必须将错误转换为用户领域 `ErrUserAlreadyExists`
- **Then** service 必须将 `ErrUserAlreadyExists` 映射为 conflict 应用错误
- **Then** 系统返回 HTTP 409 和统一失败响应信封
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `40000`，`message` 为 `用户已存在`

### Requirement: Preserve create user persistence constraints
系统必须复用 Ent `User` schema 和 `users` 表作为创建用户的数据结构来源，只有在现有 schema 无法表达创建约束时才允许通过 Ent schema 和 Atlas migration 做最小变更。

#### Scenario: Create user through Ent repository
- **Given** 创建用户请求已通过校验和业务检查
- **When** repository 写入用户记录
- **Then** repository 使用具名 Ent client `user_db`
- **Then** repository 必须设置 `user_id`、`nickname`、`username`、`status` 和 `password_hash`
- **Then** repository 不得设置或引用 `name`、`active` 或持久化字段 `password`
- **Then** `created_at` 和 `updated_at` 必须由 Ent schema 默认值写入毫秒级 Unix 时间戳
- **Then** `deleted_at` 必须保持为 `NULL`
- **Then** 不得绕过 repository 直接在 controller 中访问数据库

#### Scenario: Preserve generated code workflow
- **Given** 实现创建用户时需要调整 Ent schema
- **When** 开发者完成 schema 修改
- **Then** 必须在 `user-services` 模块运行 `go generate ./ent`
- **Then** 必须通过 Atlas 脚本生成并校验 migration
- **Then** 不得手写 `user-services/ent/` 下的生成代码

### Requirement: Initialize user token version on creation

系统 SHALL 在创建用户时初始化认证 token 版本。新用户的 `token_version` MUST 从 `1` 开始；创建用户 API 的成功响应 MUST 保持不返回密码或密码哈希，并不得要求客户端传入 `token_version`。

#### Scenario: Created user has default token version
- **Given** 调用方提交有效创建用户请求
- **When** 系统持久化新用户
- **Then** PostgreSQL 用户记录的 `token_version` MUST 为 `1`
- **Then** 创建用户响应 MUST NOT 包含 `password` 或 `token_version`

#### Scenario: Client cannot set token version during user creation
- **Given** 调用方在创建用户请求中携带 `token_version`
- **When** 系统处理创建用户请求
- **Then** 系统 MUST 忽略客户端提供的 `token_version`
- **Then** 新用户的 `token_version` MUST 使用服务端默认值 `1`

### Requirement: Protect password from user profile responses
系统必须把外部提交的 `password` 作为创建输入处理，并只在服务端转换后写入 `password_hash`，不得在创建用户成功响应、查询用户响应、业务日志或错误消息中公开密码值或密码哈希值。

#### Scenario: Create response excludes password and password hash
- **Given** 创建用户请求包含合法 `password`
- **When** 系统成功创建用户并返回响应
- **Then** 响应 `data` 不得包含 `password`
- **Then** 响应 `data` 不得包含 `password_hash`
- **Then** 业务日志不得记录 `password` 的明文值或 `password_hash` 的完整值

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

### Requirement: Create users with status enum contract
系统 MUST 将用户状态定义为用户域枚举，允许值仅为 `100` 正常、`200` 冻结/停用、`300` 必须修改密码；所有创建用户请求中的 `status` 参数 MUST 使用共享 enum 校验规则。

#### Scenario: Accept valid create status
- **Given** 创建用户请求包含 `status=100`、`status=200` 或 `status=300`
- **When** controller 校验请求 DTO
- **Then** DTO 的状态枚举 `IsValid()` MUST 返回 true
- **Then** 共享 `validateEnum` MUST 判定校验通过

#### Scenario: Reject create status without custom validation duplication
- **Given** 创建用户请求包含无效 `status`
- **When** controller 校验请求 DTO
- **Then** controller 和 service MUST NOT 使用重复硬编码列表执行 status 校验
- **Then** 请求 MUST 由共享 `validateEnum` 判定为校验失败

### Requirement: User creation data access uses repository abstraction with PostgreSQL implementation boundary
用户创建能力 SHALL 通过根 `repository.UserRepository` 抽象创建用户并检查用户名唯一性，具体 Ent/PostgreSQL 写入和查询实现 MUST 位于 `user-services/internal/repository/postgres` 包。根 `repository` 包 MUST 保留创建输入类型并 MUST NOT 依赖具体实现包。

#### Scenario: Create flow remains layered
- **Given** 用户创建 controller 已完成请求绑定和校验
- **When** service 编排用户创建
- **Then** service MUST 通过 `repository.UserRepository` 调用创建和唯一性检查
- **Then** service MUST NOT 直接调用 Ent client 或 `repository/postgres` 私有实现类型

#### Scenario: Create API remains compatible after implementation split
- **Given** `repository/postgres` 承载 Ent/PostgreSQL 用户创建实现
- **When** 调用方提交有效用户创建请求
- **Then** 系统 MUST 保持现有成功响应信封和用户响应字段
- **Then** 用户名冲突、校验失败和持久化错误的公开错误语义 MUST 与迁移前保持一致
