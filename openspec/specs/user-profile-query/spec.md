# user-profile-query

## Purpose

用户资料查询能力允许 API 调用方通过用户 ID 获取用户基础资料，并把参数错误、用户不存在和内部查询错误转换为统一响应契约。
## Requirements
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

### Requirement: Preserve user model constraints

系统必须以 Ent `User` schema 作为用户资料查询的数据结构来源，并确保查询响应只公开用户资料字段，不公开敏感凭据字段或软删除内部字段。

#### Scenario: User schema defines stable fields
- **Given** 用户资料由 Ent `User` schema 定义
- **When** 系统读取用户记录并映射响应 DTO
- **Then** 用户 `user_id` 为唯一且不可变的 UUID 字符串
- **Then** 用户 `username` 非空、唯一且最大长度为 255
- **Then** 用户 `nickname` 非空且最大长度为 128
- **Then** 用户 `password_hash` 非空且不得映射到查询响应 DTO
- **Then** 用户 `status` 必须为 `100`、`200` 或 `300`
- **Then** 用户 `deleted_at` 为 nullable 毫秒级 Unix 时间戳，`NULL` 表示未删除
- **Then** 响应包含 `status`、`created_at`、`updated_at`
- **Then** 响应不得包含 `active`、`name`、`password`、`password_hash` 或 `deleted_at`
- **Then** `created_at` 和 `updated_at` 必须为毫秒级 Unix 时间戳

#### Scenario: Repository returns unexpected database error
- **Given** 数据库查询用户时发生非 not found 错误
- **When** service 处理 repository 返回的错误
- **Then** 错误被转换为内部错误
- **Then** API 响应使用内部错误业务码 `90000` 和 `internal server error`，不暴露底层数据库细节

### Requirement: Document query user API in Swagger
系统必须为现有 `GET /api/v1/users/:user_id` 查询用户接口提供与实际路由、请求参数和统一响应契约一致的 Swagger 注解和文档输出，不得改变查询接口运行时行为。

#### Scenario: Query endpoint appears in Swagger docs
- **Given** Swagger 文档已生成
- **When** 调用方查看用户接口分组
- **Then** 文档包含 `GET /users/{user_id}` 查询用户接口
- **Then** 文档包含 `user_id` 路径参数且说明其必须为 UUID 字符串
- **Then** 文档描述 HTTP 200 成功响应为统一响应信封包装的用户资料
- **Then** 文档描述用户资料包含 `user_id`、`nickname`、`username`、`status`、`created_at`、`updated_at`
- **Then** 文档描述 `created_at` 和 `updated_at` 为毫秒级 Unix 时间戳
- **Then** 文档不得把 `name`、`active`、`password`、`password_hash` 或 `deleted_at` 描述为查询响应字段

#### Scenario: Query endpoint documents failures
- **Given** Swagger 文档已生成
- **When** 调用方查看 `GET /users/{user_id}` 响应定义
- **Then** 文档包含 HTTP 400 参数错误响应
- **Then** 文档包含 HTTP 404 用户不存在响应
- **Then** 文档包含 HTTP 500 内部错误响应

### Requirement: User profile query naming cleanup preserves layered behavior
用户资料查询相关命名标准化 SHALL 保持 controller/service/repository 分层职责不变，并不得改变 `GET /api/v1/users/:user_id` 的请求解析、业务编排、数据访问、错误映射或响应内容。

#### Scenario: Internal user query symbols are renamed
- **WHEN** 实现重命名用户查询相关内部函数、方法、参数、mapper 或类型
- **THEN** controller 仍 MUST 只处理 HTTP 解析和响应输出，service 仍 MUST 负责编排，repository 仍 MUST 负责数据库访问

#### Scenario: User query API remains compatible
- **WHEN** 命名标准化完成
- **THEN** `GET /api/v1/users/:user_id` 的路径、响应 envelope、用户响应 JSON 字段和错误语义 MUST 保持不变

### Requirement: User query data access uses repository abstraction with PostgreSQL implementation boundary
用户资料查询能力 SHALL 通过根 `repository.UserRepository` 抽象读取用户资料，具体 Ent/PostgreSQL 查询实现 MUST 位于 `user-services/internal/repository/postgres` 包。根 `repository` 包 MUST NOT 依赖 `repository/postgres`，查询 API 的路由、认证要求、错误映射和响应内容 MUST 保持不变。

#### Scenario: Query service depends on repository abstraction
- **Given** 用户资料查询 service 需要按外部 UUID 读取用户
- **When** service 调用数据访问层
- **Then** service MUST 依赖 `repository.UserRepository`
- **Then** service MUST NOT 直接依赖 `repository/postgres` 或 Ent 查询实现类型

#### Scenario: PostgreSQL implementation preserves query behavior
- **Given** `repository/postgres` 提供 `UserRepository` 的 Ent/PostgreSQL 实现
- **When** 调用方请求 `GET /api/v1/users/:user_id`
- **Then** 系统 MUST 继续只返回未软删除用户
- **Then** Ent not found MUST 继续转换为用户领域 `ErrUserNotFound`
- **Then** HTTP 响应路径、响应信封和公开字段 MUST 与迁移前保持一致

### Requirement: Validate and normalize query input before service business flow
系统 MUST 在用户查询 Service 执行业务编排前完成查询请求的请求级清洗和基础校验。路径 `user_id` 的 UUID 格式校验、列表分页归一化和过滤字段空白裁剪 MUST 位于 Controller、共享请求校验器或服务内 validators 层，而不是作为用户查询 Service 的主要职责。

#### Scenario: Validate user ID before service lookup
- **Given** 调用方请求 `GET /api/v1/users/:user_id`
- **When** `user_id` 不是合法 UUID 或缺失
- **Then** 请求 MUST 在调用 Repository 查询前被判定为参数错误
- **Then** controller 或服务内 validators 层 MUST 负责 UUID 格式校验
- **Then** Service MUST 保留用户不存在和内部查询错误的业务错误映射

#### Scenario: Pass normalized user ID to service
- **Given** 调用方请求 `GET /api/v1/users/018f0000-0000-7000-8000-000000000001`
- **When** controller 调用查询用户 Service
- **Then** Service MUST 接收已通过基础格式校验的用户 ID 输入
- **Then** Service MUST NOT 将路径参数解析错误作为主要业务分支

#### Scenario: Normalize list filters before repository access
- **Given** 调用方请求用户列表并提交 `nickname`、`username`、`page` 或 `page_size` 查询参数
- **When** controller 调用列表查询 Service
- **Then** 过滤字段空白裁剪和分页归一化 MUST 在 Controller 或服务内 validators 层完成
- **Then** Service MUST 使用规范化后的分页和过滤条件编排 Repository 查询

### Requirement: Query service uses domain user model
用户资料查询能力 SHALL 通过根 `repository.UserRepository` 获取用户领域实体，并将领域实体映射为查询响应 DTO。Service 层 MUST NOT 直接依赖 Ent 用户模型或 Ent 查询实现类型，查询 API 的认证要求、参数校验、错误映射和响应内容 MUST 保持不变。

#### Scenario: Query maps domain user to response
- **Given** PostgreSQL repository 读取到未软删除用户并返回用户领域实体
- **When** `UserService.GetUserByID` 处理查询结果
- **Then** Service MUST 将用户领域实体映射为 `dto.UserResponse`
- **Then** 响应 MUST 继续包含 `user_id`、`nickname`、`username`、`status`、`created_at` 和 `updated_at`
- **Then** 响应 MUST NOT 包含 `password_hash`、内部 `id` 或 `deleted_at`

#### Scenario: Query service remains independent of Ent
- **Given** 用户资料查询 Service 编译
- **When** 检查 Service 层依赖
- **Then** `user-services/internal/service` MUST NOT 为用户资料查询导入 `github.com/aegiscore/user-services/ent`
- **Then** Ent 查询和 Ent 到 Domain 映射 MUST 位于 `user-services/internal/repository/postgres`

#### Scenario: Query not found mapping remains unchanged
- **Given** PostgreSQL repository 未找到未软删除用户
- **When** Repository 返回 `domain.ErrUserNotFound`
- **Then** Service MUST 继续将该领域错误映射为用户不存在响应
- **Then** HTTP 404 响应信封和公开错误消息 MUST 与现有查询能力保持一致

### Requirement: User profile query depends on profile repository interface

用户资料查询服务 SHALL 仅依赖用户资料相关仓储接口读取用户资料。该接口 MUST 覆盖按外部用户 ID 查询和用户列表查询所需方法，MUST NOT 要求用户资料查询服务依赖认证凭证更新、按用户名认证读取或 token version 递增能力。查询 API 的认证要求、参数校验、错误映射、响应信封和公开字段 MUST 保持不变。

#### Scenario: Query service declares minimum repository dependency
- **Given** 用户资料查询 service 需要按外部 UUID 读取用户
- **When** service 构造函数声明仓储依赖
- **Then** service MUST 依赖用户资料仓储接口
- **Then** service MUST NOT 依赖包含认证凭证和 token version 操作的完整用户仓储大接口
- **Then** service MUST NOT 直接依赖 `repository/postgres` 或 Ent 查询实现类型

#### Scenario: Query API behavior remains compatible
- **Given** PostgreSQL 用户仓储实现通过用户资料仓储接口提供查询能力
- **When** 调用方请求 `GET /api/v1/users/:user_id`
- **Then** 系统 MUST 继续只返回未软删除用户
- **Then** Ent not found MUST 继续转换为用户领域 `ErrUserNotFound`
- **Then** HTTP 响应路径、响应信封、错误语义和公开字段 MUST 与迁移前保持一致

#### Scenario: Query service test fake stays focused
- **Given** 单元测试只验证按用户 ID 查询或用户列表查询逻辑
- **When** 测试构造用户资料查询 service 的仓储替身
- **Then** 测试替身 MUST 只需要实现用户资料查询相关方法
- **Then** 测试替身 MUST NOT 为认证凭证更新或 token version 递增提供无关空实现
