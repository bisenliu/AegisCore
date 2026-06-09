# user-profile-create

## Purpose

用户资料创建能力允许 API 调用方创建用户基础资料，并将请求校验、业务校验、唯一性冲突和持久化错误转换为统一响应契约。
## Requirements
### Requirement: Create user with validated JSON request
系统必须通过 `POST /api/v1/users` 创建用户，并要求调用方提供有效的 Bearer token。认证通过后，controller 必须使用 `common/validation` 绑定并校验 JSON 请求体，成功时必须返回统一成功响应信封和创建后的用户资料。认证失败时，系统必须在进入 controller 前返回统一未认证响应。

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
- **Then** 请求不得进入 `UserController.CreateUser`

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
用户创建能力 SHALL 通过 app service 消费侧声明的用户资料持久化端口创建用户，具体 Ent/PostgreSQL 写入和查询实现 MUST 位于 `user-services/internal/features/user/infra/postgres` 包。用户创建 service 的创建输入模型 MUST 由消费侧声明，根 `repository` 包 MUST NOT 定义用户资料创建 service 消费的接口或输入模型。

#### Scenario: Create flow remains layered
- **Given** 用户创建 controller 已完成请求绑定和校验
- **When** service 编排用户创建
- **Then** service MUST 通过 `user-services/internal/features/user/app` 声明的用户资料持久化端口调用创建
- **Then** service MUST NOT 调用 `ExistsByUsername` 或等价用户名存在性预查
- **Then** service MUST NOT 直接调用 Ent client 或 `features/user/infra/postgres` 私有实现类型

#### Scenario: Create API remains compatible after implementation split
- **Given** `features/user/infra/postgres` 承载 Ent/PostgreSQL 用户创建实现
- **When** 调用方提交有效用户创建请求
- **Then** 系统 MUST 保持现有成功响应信封和用户响应字段
- **Then** 用户名冲突、校验失败和持久化错误的公开错误语义 MUST 与迁移前保持一致

### Requirement: Validate and normalize create input before service business flow
系统 MUST 在用户创建 Service 执行业务编排前完成创建请求的请求级清洗和基础校验。`nickname`、`username` 和 `password` 的空白裁剪、`username` 小写规范化、必填校验、长度/格式校验和状态枚举校验 MUST 位于 Controller、共享请求校验器或服务内 validators 层，而不是作为用户创建 Service 的主要职责。

#### Scenario: Trim and lowercase create user fields before service call
- **Given** 调用方提交 `nickname`、`username` 或 `password` 前后包含空白的创建用户请求，且 `username` 包含大写字母
- **When** controller 处理创建用户请求并调用 Service
- **Then** 空白裁剪 MUST 在 Controller 或服务内 validators 层完成
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

### Requirement: Create flow returns domain user model
用户创建能力 SHALL 在持久化创建成功后由 Repository 返回用户领域实体，Service 层 MUST 将该领域实体映射为创建响应 DTO。创建流程中的请求校验、用户名唯一性检查、密码 hash、UUIDv7 生成、冲突错误映射和成功响应内容 MUST 保持不变。

#### Scenario: Create repository returns domain user
- **Given** Service 已完成创建用户业务编排并调用 Repository 写入用户记录
- **When** PostgreSQL repository 使用 Ent 创建用户成功
- **Then** Repository MUST 将创建后的 Ent 用户模型转换为用户领域实体
- **Then** Repository MUST 返回用户领域实体给 Service
- **Then** Service MUST NOT 直接读取 Ent 用户模型构造创建响应

#### Scenario: Create response remains compatible
- **Given** 创建用户成功并获得用户领域实体
- **When** Service 构造创建响应
- **Then** 响应 MUST 继续返回 HTTP 201 和统一成功响应信封
- **Then** 响应 `data` MUST 继续包含 `user_id`、`nickname`、`username`、`status`、`created_at` 和 `updated_at`
- **Then** 响应 `data` MUST NOT 包含 `password`、`password_hash`、`token_version`、内部 `id` 或 `deleted_at`

#### Scenario: Create uniqueness errors remain domain errors
- **Given** Ent 创建用户时发生唯一约束冲突
- **When** PostgreSQL repository 处理该持久化错误
- **Then** Repository MUST 继续返回 `domain.ErrUserAlreadyExists`
- **Then** Service MUST 继续将该领域错误映射为用户已存在冲突响应

### Requirement: Use explicit user creation handler names
用户资料创建能力 SHALL 在 controller 和路由注册中使用能独立表达用户创建语义的 handler 名称。实现 MUST 保持 `POST /api/v1/users` 的请求体校验、认证要求、响应信封、HTTP 201 成功语义、冲突错误语义和分层职责不变。

#### Scenario: User creation route uses explicit handler name
- **Given** 用户创建路由已注册
- **When** 开发者检查 `POST /api/v1/users` 的 Gin handler 引用
- **Then** 路由 MUST 引用 `UserController.CreateUser`
- **Then** 路由 MUST NOT 引用 `UserController.Create`

#### Scenario: User creation API behavior remains unchanged
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `POST /api/v1/users` 并提交合法 JSON 请求体
- **Then** 系统 MUST 继续创建用户并返回 HTTP 201
- **Then** 请求绑定、输入清洗、service 编排和 repository 持久化职责 MUST 保持不变

### Requirement: User creation depends on profile repository interface

用户资料创建服务 SHALL 仅依赖由消费方声明的用户资料相关仓储接口创建用户资料。该接口 MUST 覆盖创建用户、按外部用户 ID 查询和用户列表查询等用户资料服务实际消费的方法，MUST NOT 要求创建服务依赖认证凭证更新、按用户名认证读取或 token version 递增能力。创建 API 的请求校验、密码 hash、用户名唯一性冲突映射、响应信封和公开字段 MUST 保持不变。

#### Scenario: Create service declares minimum repository dependency
- **Given** 用户创建 service 需要持久化新用户记录
- **When** service 构造函数声明仓储依赖
- **Then** service MUST 依赖由 `user-services/internal/features/user/app` 声明的用户资料仓储接口
- **Then** service MUST NOT 依赖包含认证凭证和 token version 操作的完整用户仓储大接口
- **Then** service MUST NOT 直接调用 Ent client 或 `features/user/infra/postgres` 私有实现类型

#### Scenario: Create API behavior remains compatible
- **Given** PostgreSQL 用户仓储实现通过用户资料仓储接口提供创建能力
- **When** 调用方提交有效用户创建请求
- **Then** 系统 MUST 继续创建用户并返回现有成功响应信封和用户响应字段
- **Then** 用户名冲突、校验失败和持久化错误的公开错误语义 MUST 与迁移前保持一致
- **Then** 创建响应 MUST NOT 包含 `password`、`password_hash`、`token_version`、内部 `id` 或 `deleted_at`

#### Scenario: Create service test fake stays focused
- **Given** 单元测试只验证用户创建流程
- **When** 测试构造用户创建 service 的仓储替身
- **Then** 测试替身 MUST 只需要实现用户资料创建和资料读取相关方法
- **Then** 测试替身 MUST NOT 为认证凭证更新、按用户名认证读取或 token version 递增提供无关空实现

### Requirement: User creation API contracts are grouped by capability

用户资料创建能力 SHALL 使用按业务能力组织的用户 API 契约包承载创建请求和用户响应模型。实现 MUST NOT 依赖全局 DTO 包表达用户创建契约，并 MUST 保持 `POST /api/v1/users` 的外部 HTTP 行为不变。

#### Scenario: Create user contract types use user API package
- **WHEN** controller、service、validation 或测试引用创建用户请求或创建成功用户响应
- **THEN** 这些引用 MUST 来自用户 API 契约包
- **THEN** 这些引用 MUST NOT 来自全局 DTO 包

#### Scenario: Create user request contract remains compatible
- **WHEN** 用户创建请求类型迁移完成
- **THEN** 请求体 MUST 继续使用 `nickname`、`username`、`password` 和可选 `status` 字段
- **THEN** 原有 JSON tag、校验 tag、label 和 example 语义 MUST 保持不变
- **THEN** 缺省用户状态和请求级规范化行为 MUST 保持不变

#### Scenario: Create user response contract remains compatible
- **WHEN** 用户创建响应类型迁移完成
- **THEN** `POST /api/v1/users` MUST 继续返回 HTTP 201 和统一成功响应信封
- **THEN** 创建响应 MUST 继续包含 `user_id`、`nickname`、`username`、`status`、`created_at` 和 `updated_at`
- **THEN** 创建响应 MUST NOT 包含 `password`、`password_hash`、`token_version`、内部 `id` 或 `deleted_at`

