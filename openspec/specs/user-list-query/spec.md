# user-list-query

## Purpose

用户列表查询能力允许 API 调用方分页获取用户资料列表，按明确白名单字段过滤用户，并通过统一分页响应契约返回结果。
## Requirements
### Requirement: List users with pagination

系统 MUST 提供 `GET /api/v1/users` 用户列表接口。接口 MUST 支持分页查询，MUST 通过统一响应信封返回分页用户资料列表，且返回用户资料 MUST 不包含 `password` 或 `password_hash`。列表默认只返回 `deleted_at IS NULL` 的用户。

#### Scenario: List users with explicit pagination
- **Given** 数据库中存在多个未软删除用户记录
- **When** 调用方请求 `GET /api/v1/users?page=1&page_size=20`
- **Then** 系统返回 HTTP 200
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `0`，`message` 为 `ok`
- **Then** `data.items` MUST 为用户资料数组
- **Then** 每个用户资料 MUST 包含 `user_id`、`nickname`、`username`、`status`、`created_at`、`updated_at`
- **Then** 每个用户资料 MUST NOT 包含 `password`、`password_hash`、`name`、`active` 或 `deleted_at`
- **Then** `data.pagination.page` MUST 为 `1`
- **Then** `data.pagination.page_size` MUST 为 `20`
- **Then** `data.pagination.total` MUST 为过滤后的未软删除记录总数
- **Then** `data.pagination.total_pages` MUST 为基于 `total` 和 `page_size` 向上取整的页数

#### Scenario: List users with default pagination
- **Given** 调用方未提供分页参数
- **When** 调用方请求 `GET /api/v1/users`
- **Then** 系统 MUST 按 `page=1` 和 `page_size=10` 查询
- **Then** 响应 `data.pagination.page` MUST 为 `1`
- **Then** 响应 `data.pagination.page_size` MUST 为 `10`

#### Scenario: List users with invalid pagination boundaries
- **Given** 调用方提供的 `page` 小于 `1` 或 `page_size` 小于 `1`
- **When** 调用方请求用户列表接口
- **Then** 系统 MUST 将小于 `1` 的 `page` 视为 `1`
- **Then** 系统 MUST 将小于 `1` 的 `page_size` 视为 `10`

#### Scenario: List users returns empty page
- **Given** 数据库中没有匹配当前过滤条件的未软删除用户记录
- **When** 调用方请求用户列表接口
- **Then** 系统返回 HTTP 200
- **Then** `data.items` MUST 为空数组
- **Then** `data.pagination.total` MUST 为 `0`
- **Then** `data.pagination.total_pages` MUST 为 `0`

#### Scenario: Soft deleted users are excluded from list
- **Given** 数据库中存在 `deleted_at` 不为 `NULL` 的用户记录
- **When** 调用方请求用户列表接口
- **Then** repository 查询和 count MUST 添加 `deleted_at IS NULL` 条件
- **Then** 响应列表和分页总数 MUST 不包含软删除用户

### Requirement: Filter listed users

用户列表接口 MUST 支持明确白名单过滤参数。系统 MUST 支持 `nickname`、`username`、`status` 过滤，并忽略未定义的 query 参数。所有 `status` 查询参数 MUST 使用共享 enum 校验规则。

#### Scenario: Filter users by nickname
- **Given** 数据库中存在不同昵称的未软删除用户
- **When** 调用方请求 `GET /api/v1/users?nickname=Ali`
- **Then** 系统 MUST 返回 `nickname` 包含 `Ali` 的未软删除用户
- **Then** `data.pagination.total` MUST 反映昵称过滤后的未软删除记录总数

#### Scenario: Filter users by username
- **Given** 数据库中存在用户名为 `alice` 的未软删除用户
- **When** 调用方请求 `GET /api/v1/users?username=alice`
- **Then** 系统 MUST 以清洗后的用户名进行精确匹配
- **Then** 返回结果 MUST 只包含用户名为 `alice` 且未软删除的用户

#### Scenario: Filter users by status
- **Given** 数据库中同时存在不同 `status` 的未软删除用户
- **When** 调用方请求 `GET /api/v1/users?status=100`
- **Then** 系统 MUST 只返回 `status` 为 `100` 且未软删除的用户

#### Scenario: Reject invalid status filter
- **Given** 调用方提供不属于 `100`、`200` 或 `300` 的 `status` 参数
- **When** 调用方请求用户列表接口
- **Then** 查询 DTO 的 `status` 字段 MUST 通过 `validate:"enum"` 触发共享 `validateEnum`
- **Then** 系统 MUST 返回 HTTP 400
- **Then** 响应 MUST 使用统一失败信封

### Requirement: Preserve layered user list behavior

用户列表实现 MUST 保持 controller/service/repository 分层职责。HTTP query 解析 MUST 位于 controller，过滤条件清洗和分页业务编排 MUST 位于 service，Ent 查询与 count MUST 位于 repository。实现 MUST 移除列表查询中的 email 输入字段、email 清洗逻辑和 email Ent 谓词。

#### Scenario: Repository returns unexpected list error
- **Given** 数据库执行用户列表查询或 count 时发生非预期错误
- **When** service 处理 repository 返回的错误
- **Then** 错误 MUST 被转换为内部错误
- **Then** API 响应 MUST 使用内部错误业务码 `90000` 和 `internal server error`
- **Then** 响应 MUST NOT 暴露底层数据库细节

#### Scenario: Repository lists by username instead of email
- **Given** 用户列表请求包含 `username` 过滤条件
- **When** repository 构造 Ent 查询谓词
- **Then** repository MUST 使用 `username` 字段谓词
- **Then** repository MUST NOT 引用 `email` 字段、`EmailEQ` 谓词或 email 输入字段

### Requirement: Document user list API in Swagger

系统 MUST 为 `GET /api/v1/users` 用户列表接口提供与运行时行为一致的 Swagger/OpenAPI 注解和文档输出。

#### Scenario: List endpoint appears in Swagger docs
- **Given** Swagger 文档已生成
- **When** 调用方查看用户接口分组
- **Then** 文档 MUST 包含 `GET /users` 用户列表接口
- **Then** 文档 MUST 描述 `page`、`page_size`、`nickname`、`username`、`status` query 参数
- **Then** 文档 MUST 描述 `status` 允许值为 `100`、`200`、`300`
- **Then** 文档 MUST 描述 HTTP 200 成功响应为统一响应信封包装的分页用户资料
- **Then** 文档 MUST 不得把 `name`、`active`、`password`、`password_hash` 或 `deleted_at` 描述为响应字段

### Requirement: User list query uses repository abstraction with PostgreSQL implementation boundary
用户列表查询能力 SHALL 通过 `userapp.UserProfileStore` 抽象执行分页、过滤和排序相关数据访问，具体 Ent/PostgreSQL predicate 与查询实现 MUST 位于 `user-services/internal/features/user/store/postgres` 包。实现 MUST NOT 新增根 `repository` 包承载列表查询输入类型或依赖具体实现包。

#### Scenario: List service remains decoupled from Ent query implementation
- **Given** 用户列表 service 需要查询用户列表
- **When** service 调用数据访问层
- **Then** service MUST 通过 `userapp.UserProfileStore.ListUsers` 提交列表查询输入
- **Then** service MUST NOT 直接引用 Ent predicate helper 或 `features/user/store/postgres` 私有实现类型

#### Scenario: List query behavior remains compatible
- **Given** `features/user/store/postgres` 承载 Ent/PostgreSQL 列表查询实现
- **When** 调用方请求用户列表 API
- **Then** 系统 MUST 继续按现有分页、过滤、排序和未软删除条件返回用户列表
- **Then** HTTP 响应信封、公开字段和错误语义 MUST 与迁移前保持一致

### Requirement: List service uses domain user collection
用户列表查询能力 SHALL 通过 `userapp.UserProfileStore` 获取用户领域实体集合和总数，并由 Service 层映射为分页响应 DTO。Service 层 MUST NOT 直接依赖 Ent 用户列表模型，列表 API 的过滤、分页、响应信封和公开字段 MUST 保持不变。

#### Scenario: List repository returns domain users
- **Given** Service 使用规范化后的分页和过滤条件调用用户 Repository
- **When** PostgreSQL repository 查询用户列表成功
- **Then** Repository MUST 将 Ent 用户模型集合转换为用户领域实体集合
- **Then** Repository MUST 返回用户领域实体集合和总数
- **Then** Service MUST NOT 遍历 `[]*ent.User` 构造列表响应

#### Scenario: List response remains compatible
- **Given** Service 获得用户领域实体集合和总数
- **When** Service 构造分页响应
- **Then** 响应 items MUST 继续只包含公开用户资料字段
- **Then** 响应 pagination MUST 继续使用现有页码、每页数量和总数语义
- **Then** 响应 MUST NOT 包含 `password_hash`、内部 `id` 或 `deleted_at`

#### Scenario: List service remains independent of Ent
- **Given** 用户列表查询 Service 编译
- **When** 检查 Service 层依赖
- **Then** `user-services/internal/features/user/app` MUST NOT 为用户列表查询导入 `github.com/aegiscore/user-services/ent`
- **Then** Ent 列表查询和 Ent 到 Domain 映射 MUST 位于 `user-services/internal/features/user/store/postgres`

### Requirement: Use explicit user list handler names
用户列表查询能力 SHALL 在 controller 和路由注册中使用能独立表达用户列表语义的 handler 名称。实现 MUST 保持 `GET /api/v1/users` 的请求参数、认证要求、响应信封、分页语义、错误语义和分层职责不变。

#### Scenario: User list route uses explicit handler name
- **Given** 用户列表路由已注册
- **When** 开发者检查 `GET /api/v1/users` 的 Gin handler 引用
- **Then** 路由 MUST 引用 `UserController.ListUsers`
- **Then** 路由 MUST NOT 引用 `UserController.List`

#### Scenario: User list API behavior remains unchanged
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users`
- **Then** 系统 MUST 继续通过统一响应信封返回分页用户列表
- **Then** controller、service 和 repository 的职责边界 MUST 保持不变
