## MODIFIED Requirements

### Requirement: List users with pagination

系统 MUST 提供 `GET /api/v1/users` 用户列表接口。接口 MUST 支持基于 `user_id` 的 keyset pagination，MUST 通过统一响应信封返回分页用户资料列表，且返回用户资料 MUST 不包含 `password` 或 `password_hash`。列表默认只返回 `deleted_at IS NULL` 的用户，并 MUST 按 `user_id ASC` 稳定排序。

#### Scenario: List users with explicit keyset pagination
- **Given** 数据库中存在多个未软删除用户记录
- **When** 调用方请求 `GET /api/v1/users?cursor=<user_id>&page_size=20`
- **Then** 系统返回 HTTP 200
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `0`，`message` 为 `ok`
- **Then** `data.items` MUST 为用户资料数组
- **Then** 每个用户资料 MUST 包含 `user_id`、`nickname`、`username`、`status`、`created_at`、`updated_at`
- **Then** 每个用户资料 MUST NOT 包含 `password`、`password_hash`、`name`、`active` 或 `deleted_at`
- **Then** 查询 MUST 只返回 `user_id` 大于 `cursor` 的未软删除用户
- **Then** 查询 MUST 按 `user_id ASC` 排序
- **Then** `data.pagination.page_size` MUST 为 `20`
- **Then** `data.pagination` MUST 只包含 `page_size`、`next_cursor`、`has_next`
- **Then** `data.pagination` MUST NOT 包含 `page`、`offset`、`total` 或 `total_pages`

#### Scenario: List users with default keyset pagination
- **Given** 调用方未提供分页参数
- **When** 调用方请求 `GET /api/v1/users`
- **Then** 系统 MUST 按 `page_size=10` 查询首页
- **Then** 响应 `data.pagination.page_size` MUST 为 `10`
- **Then** 响应 `data.pagination.next_cursor` MUST 为空或省略，除非存在下一页
- **Then** 响应 `data.pagination.has_next` MUST 表示是否存在下一页

#### Scenario: List users with invalid page size boundaries
- **Given** 调用方提供的 `page_size` 小于 `1` 或大于 `100`
- **When** 调用方请求用户列表接口
- **Then** 系统 MUST 将小于 `1` 的 `page_size` 视为 `10`
- **Then** 系统 MUST 将大于 `100` 的 `page_size` 视为 `100`
- **Then** repository 查询 limit MUST 使用规范化后的 `page_size`

#### Scenario: Reject invalid list cursor
- **Given** 调用方提供的 `cursor` 不是合法 UUID
- **When** 调用方请求用户列表接口
- **Then** 系统 MUST 返回 HTTP 400
- **Then** 响应 MUST 使用统一失败信封
- **Then** 系统 MUST NOT 执行用户列表 repository 查询

#### Scenario: List users returns empty page
- **Given** 数据库中没有匹配当前过滤条件或 cursor 之后的未软删除用户记录
- **When** 调用方请求用户列表接口
- **Then** 系统返回 HTTP 200
- **Then** `data.items` MUST 为空数组
- **Then** `data.pagination.page_size` MUST 为规范化后的 page size
- **Then** `data.pagination.next_cursor` MUST 为空或省略
- **Then** `data.pagination.has_next` MUST 为 `false`

#### Scenario: Soft deleted users are excluded from list
- **Given** 数据库中存在 `deleted_at` 不为 `NULL` 的用户记录
- **When** 调用方请求用户列表接口
- **Then** repository 查询 MUST 添加 `deleted_at IS NULL` 条件
- **Then** 响应列表和 `has_next` 判断 MUST 不包含软删除用户

#### Scenario: Next cursor is produced only when another page exists
- **Given** repository 返回当前页用户集合和 `hasNext=true`
- **When** service 构造用户列表结果
- **Then** `next_cursor` MUST 等于当前页最后一条用户的 `user_id` 字符串
- **Then** `has_next` MUST 为 `true`

#### Scenario: No next cursor when there is no next page
- **Given** repository 返回当前页用户集合和 `hasNext=false`
- **When** service 构造用户列表结果
- **Then** `next_cursor` MUST 为空
- **Then** `has_next` MUST 为 `false`

### Requirement: Filter listed users

用户列表接口 MUST 支持明确白名单过滤参数。系统 MUST 支持 `nickname`、`username`、`status` 过滤，并忽略未定义的 query 参数。所有 `status` 查询参数 MUST 使用共享 enum 校验规则。过滤条件 MUST 与 keyset pagination 共同生效。

#### Scenario: Filter users by nickname
- **Given** 数据库中存在不同昵称的未软删除用户
- **When** 调用方请求 `GET /api/v1/users?nickname=Ali`
- **Then** 系统 MUST 返回 `nickname` 包含 `Ali` 的未软删除用户
- **Then** 返回结果 MUST 按 `user_id ASC` 排序
- **Then** `data.pagination.has_next` MUST 只反映昵称过滤后的未软删除记录是否存在下一页

#### Scenario: Filter users by username
- **Given** 数据库中存在用户名为 `alice` 的未软删除用户
- **When** 调用方请求 `GET /api/v1/users?username=alice`
- **Then** 系统 MUST 以清洗后的用户名进行精确匹配
- **Then** 返回结果 MUST 只包含用户名为 `alice` 且未软删除的用户

#### Scenario: Filter users by status
- **Given** 数据库中同时存在不同 `status` 的未软删除用户
- **When** 调用方请求 `GET /api/v1/users?status=100`
- **Then** 系统 MUST 只返回 `status` 为 `100` 且未软删除的用户
- **Then** 返回结果 MUST 按 `user_id ASC` 排序

#### Scenario: Reject invalid status filter
- **Given** 调用方提供不属于 `100`、`200` 或 `300` 的 `status` 参数
- **When** 调用方请求用户列表接口
- **Then** 查询 DTO 的 `status` 字段 MUST 通过 `validate:"enum"` 触发共享 `validateEnum`
- **Then** 系统 MUST 返回 HTTP 400
- **Then** 响应 MUST 使用统一失败信封

### Requirement: Preserve layered user list behavior

用户列表实现 MUST 保持 controller/service/repository 分层职责。HTTP query 解析和 cursor UUID 解析 MUST 位于 controller 或 feature-local HTTP validation，过滤条件清洗和分页业务编排 MUST 位于 service，Ent 查询、keyset predicate 和 hasNext 计算 MUST 位于 repository。实现 MUST 移除列表查询中的 page、offset、count、total 和 total_pages 逻辑。

#### Scenario: Repository returns unexpected list error
- **Given** 数据库执行用户列表查询时发生非预期错误
- **When** service 处理 repository 返回的错误
- **Then** 错误 MUST 被转换为内部错误
- **Then** API 响应 MUST 使用内部错误业务码 `90000` 和 `internal server error`
- **Then** 响应 MUST NOT 暴露底层数据库细节

#### Scenario: Repository lists by username instead of email
- **Given** 用户列表请求包含 `username` 过滤条件
- **When** repository 构造 Ent 查询谓词
- **Then** repository MUST 使用 `username` 字段谓词
- **Then** repository MUST NOT 引用 `email` 字段、`EmailEQ` 谓词或 email 输入字段

#### Scenario: Repository uses keyset query without count or offset
- **Given** service 请求用户列表
- **When** repository 构造 Ent 查询
- **Then** repository MUST 使用 `user_id > cursor` 表达 cursor 之后的数据范围
- **Then** repository MUST 使用 `user_id ASC` 排序
- **Then** repository MUST 使用 `limit + 1` 查询是否存在下一页
- **Then** repository MUST NOT 调用 `Count(ctx)`
- **Then** repository MUST NOT 调用 `Offset(...)`

### Requirement: Document user list API in Swagger

系统 MUST 为 `GET /api/v1/users` 用户列表接口提供与运行时行为一致的 Swagger/OpenAPI 注解和文档输出。

#### Scenario: List endpoint appears in Swagger docs
- **Given** Swagger 文档已生成
- **When** 调用方查看用户接口分组
- **Then** 文档 MUST 包含 `GET /users` 用户列表接口
- **Then** 文档 MUST 描述 `cursor`、`page_size`、`nickname`、`username`、`status` query 参数
- **Then** 文档 MUST NOT 描述 `page`、`offset`、`total` 或 `total_pages` 分页语义
- **Then** 文档 MUST 描述 `status` 允许值为 `100`、`200`、`300`
- **Then** 文档 MUST 描述 HTTP 200 成功响应为统一响应信封包装的分页用户资料
- **Then** 文档 MUST 不得把 `name`、`active`、`password`、`password_hash` 或 `deleted_at` 描述为响应字段

### Requirement: User list query uses repository abstraction with PostgreSQL implementation boundary
用户列表查询能力 SHALL 通过 `userapp.UserProfileStore` 抽象执行 keyset 分页、过滤和排序相关数据访问，具体 Ent/PostgreSQL predicate 与查询实现 MUST 位于 `user-services/internal/features/user/infra/postgres` 包。实现 MUST NOT 新增根 `repository` 包承载列表查询输入类型或依赖具体实现包。

#### Scenario: List service remains decoupled from Ent query implementation
- **Given** 用户列表 service 需要查询用户列表
- **When** service 调用数据访问层
- **Then** service MUST 通过 `userapp.UserProfileStore.ListUsers` 提交列表查询输入
- **Then** service MUST NOT 直接引用 Ent predicate helper 或 `features/user/infra/postgres` 私有实现类型

#### Scenario: List query behavior uses keyset pagination
- **Given** `features/user/infra/postgres` 承载 Ent/PostgreSQL 列表查询实现
- **When** 调用方请求用户列表 API
- **Then** 系统 MUST 按 keyset pagination、过滤、`user_id ASC` 排序和未软删除条件返回用户列表
- **Then** HTTP 响应信封、公开用户字段和错误语义 MUST 与用户列表能力约定一致

### Requirement: List service uses domain user collection
用户列表查询能力 SHALL 通过 `userapp.UserProfileStore` 获取用户领域实体集合和 hasNext 标记，并由 Service 层映射为分页响应 DTO。Service 层 MUST NOT 直接依赖 Ent 用户列表模型，列表 API 的过滤、keyset 分页、响应信封和公开字段 MUST 保持能力契约一致。

#### Scenario: List repository returns domain users
- **Given** Service 使用规范化后的分页和过滤条件调用用户 Repository
- **When** PostgreSQL repository 查询用户列表成功
- **Then** Repository MUST 将 Ent 用户模型集合转换为用户领域实体集合
- **Then** Repository MUST 返回用户领域实体集合和 hasNext 标记
- **Then** Service MUST NOT 遍历 `[]*ent.User` 构造列表响应

#### Scenario: List response uses keyset pagination fields
- **Given** Service 获得用户领域实体集合和 hasNext 标记
- **When** Service 构造分页响应
- **Then** 响应 items MUST 继续只包含公开用户资料字段
- **Then** 响应 pagination MUST 使用 `page_size`、`next_cursor`、`has_next` 语义
- **Then** 响应 MUST NOT 包含 `password_hash`、内部 `id`、`deleted_at`、`page`、`total` 或 `total_pages`

#### Scenario: List service remains independent of Ent
- **Given** 用户列表查询 Service 编译
- **When** 检查 Service 层依赖
- **Then** `user-services/internal/features/user/app` MUST NOT 为用户列表查询导入 `github.com/aegiscore/user-services/ent`
- **Then** Ent 列表查询和 Ent 到 Domain 映射 MUST 位于 `user-services/internal/features/user/infra/postgres`

### Requirement: Use explicit user list handler names
用户列表查询能力 SHALL 在 controller 和路由注册中使用能独立表达用户列表语义的 handler 名称。实现 MUST 保持 `GET /api/v1/users` 的认证要求、响应信封、错误语义和分层职责不变，并将分页语义改为基于 `user_id` 的 keyset pagination。

#### Scenario: User list route uses explicit handler name
- **Given** 用户列表路由已注册
- **When** 开发者检查 `GET /api/v1/users` 的 Gin handler 引用
- **Then** 路由 MUST 引用 `UserController.ListUsers`
- **Then** 路由 MUST NOT 引用 `UserController.List`

#### Scenario: User list API behavior uses keyset pagination
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users`
- **Then** 系统 MUST 继续通过统一响应信封返回分页用户列表
- **Then** controller、service 和 repository 的职责边界 MUST 保持不变
- **Then** 分页协议 MUST 使用 `cursor`、`page_size`、`next_cursor` 和 `has_next`
