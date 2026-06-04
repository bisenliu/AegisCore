## MODIFIED Requirements

### Requirement: Create user with validated JSON request
系统 MUST 通过 `POST /api/v1/users` 创建用户，并要求调用方提供有效的 Bearer token。认证通过后，controller MUST 使用 `common/validation` 绑定并校验 JSON 请求体，成功时 MUST 返回统一成功响应信封和创建后的用户资料。认证失败时，系统 MUST 在进入 controller 前返回统一未认证响应。

#### Scenario: Create user successfully
- **Given** 数据库中不存在用户名为 `alice` 的用户，包括未删除和已软删除用户
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `POST /api/v1/users` 并提交包含 `nickname`、`username`、`password` 的合法 JSON 请求体，且 `username` 为 `Alice`
- **Then** controller 使用共享校验器绑定 JSON 请求体
- **Then** 系统必须在持久化前将 `username` 统一转换为小写 `alice`
- **Then** service 必须将请求密码转换为 `password_hash` 后创建用户记录
- **Then** 新用户的 `status` 必须为 `100`
- **Then** 新用户的 `deleted_at` 必须为 `NULL`
- **Then** 系统返回 HTTP 201
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `0`，`message` 为 `created`
- **Then** `data` 包含新用户的 `user_id`、`nickname`、`username`、`status`、`created_at`、`updated_at`
- **Then** `data.username` 必须为小写规范化后的 `alice`
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

### Requirement: Enforce unique user identity
系统必须以小写规范化后的 `username` 作为创建用户的全局唯一账号名。若数据库中已存在相同 `username`，包括已软删除用户，系统必须返回统一冲突错误，不得创建重复用户。创建流程不得通过 `ExistsByUsername` 执行预查，repository MUST 将创建时发生的 Ent 或数据库唯一约束错误转换为用户领域 `ErrUserAlreadyExists`，service MUST 将该领域错误映射为现有 conflict 应用错误。

#### Scenario: Reject existing username through database constraint
- **Given** 数据库中已存在用户名为 `alice` 的用户
- **When** 调用方请求创建用户名为 `ALICE` 的用户
- **Then** 系统必须在持久化前将 `username` 统一转换为小写 `alice`
- **Then** service MUST NOT 调用 `ExistsByUsername` 或等价用户名存在性预查
- **Then** repository 写入时必须依赖数据库 `UNIQUE(username)` 约束识别冲突
- **Then** repository 必须将唯一约束错误转换为用户领域 `ErrUserAlreadyExists`
- **Then** service 必须将 `ErrUserAlreadyExists` 映射为 conflict 应用错误
- **Then** 系统返回 HTTP 409
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `40000`
- **Then** 响应信封的 `message` 使用 `user-services/internal/errmsg` 中维护的用户已存在文案

#### Scenario: Soft deleted username remains reserved
- **Given** 数据库中存在用户名为 `alice` 且 `deleted_at` 不为 `NULL` 的软删除用户
- **When** 调用方请求创建用户名为 `alice` 的用户
- **Then** service MUST NOT 因用户已软删除而释放该用户名
- **Then** repository 写入时必须依赖数据库 `UNIQUE(username)` 约束识别冲突
- **Then** repository 必须将唯一约束错误转换为用户领域 `ErrUserAlreadyExists`
- **Then** 系统返回 HTTP 409 和统一失败响应信封
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `40000`，`message` 为 `用户已存在`

#### Scenario: Convert database uniqueness violation to conflict
- **Given** 并发创建导致 `username` 或 `user_id` 唯一索引在数据库写入时冲突
- **When** repository 收到 Ent 唯一约束错误
- **Then** repository 必须将错误转换为用户领域 `ErrUserAlreadyExists`
- **Then** service 必须将 `ErrUserAlreadyExists` 映射为 conflict 应用错误
- **Then** 系统返回 HTTP 409 和统一失败响应信封
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `40000`，`message` 为 `用户已存在`

### Requirement: Preserve create user persistence constraints
系统必须复用 Ent `User` schema 和 `users` 表作为创建用户的数据结构来源，只有在现有 schema 无法表达创建约束时才允许通过 Ent schema 和 Atlas migration 做最小变更。`username` MUST 全表唯一，`nickname` MUST 仅作为可重复展示名，所有业务引用用户身份时 MUST 使用外部 `user_id`。

#### Scenario: Create user through Ent repository
- **Given** 创建用户请求已通过校验和业务检查
- **When** repository 写入用户记录
- **Then** repository 使用具名 Ent client `user_db`
- **Then** repository 必须设置 `user_id`、`nickname`、`username`、`status` 和 `password_hash`
- **Then** repository 写入的 `username` 必须为小写规范化值
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

#### Scenario: Nickname is a repeatable display name
- **Given** 数据库中已存在 `nickname` 为 `Alice` 的用户
- **When** 调用方创建另一个 `nickname` 同为 `Alice` 但 `username` 不同的用户
- **Then** 系统 MUST 允许重复 `nickname`
- **Then** 唯一性约束 MUST NOT 使用 `nickname`

#### Scenario: Business references use user id
- **Given** 创建用户成功并生成外部 UUID `user_id`
- **When** 其他业务能力需要引用该用户
- **Then** 业务引用 MUST 使用 `user_id`
- **Then** 业务引用 MUST NOT 使用 `username`、`nickname` 或内部数据库 `id` 作为跨业务引用键

### Requirement: Validate and normalize create input before service business flow
系统 MUST 在用户创建 Service 执行业务编排前完成创建请求的请求级清洗和基础校验。`nickname`、`username` 和 `password` 的空白裁剪、`username` 小写规范化、必填校验、长度/格式校验和状态枚举校验 MUST 位于 Controller、共享请求校验器或服务内 Validation 层，而不是作为用户创建 Service 的主要职责。

#### Scenario: Trim and lowercase create user fields before service call
- **Given** 调用方提交 `nickname`、`username` 或 `password` 前后包含空白的创建用户请求，且 `username` 包含大写字母
- **When** controller 处理创建用户请求并调用 Service
- **Then** 空白裁剪 MUST 在 Controller 或服务内 Validation 层完成
- **Then** `username` 小写规范化 MUST 在持久化前完成
- **Then** Service MUST 使用已规范化的 `nickname`、`username` 和 `password` 执行业务流程

#### Scenario: Reject blank create fields before persistence checks
- **Given** 创建用户请求的 `nickname`、`username` 或 `password` 在裁剪后为空
- **When** controller 处理创建用户请求
- **Then** 请求 MUST 在进入持久化创建前被判定为校验失败
- **Then** 系统 MUST 返回统一 HTTP 400 失败响应信封
- **Then** Service MUST NOT 将空值基础校验作为创建用户的主要业务分支

#### Scenario: Keep password hashing and error mapping in service
- **Given** 创建用户请求已经完成请求级校验和规范化
- **When** Service 创建用户
- **Then** Service MUST NOT 调用 `ExistsByUsername` 或等价用户名存在性预查
- **Then** Service MUST 将明文密码转换为 `password_hash`
- **Then** Service MUST 将 repository 返回的用户已存在领域错误映射为统一冲突响应错误
