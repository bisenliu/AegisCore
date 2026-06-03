## MODIFIED Requirements

### Requirement: List users with pagination

系统 MUST 提供 `GET /api/v1/users` 用户列表接口。接口 MUST 支持分页查询，MUST 通过统一响应信封返回分页用户资料列表，且返回用户资料 MUST 不包含 `password` 或 `password_hash`。列表默认只返回 `deleted_at IS NULL` 的用户。

#### Scenario: List users with explicit pagination
- **Given** 数据库中存在多个未软删除用户记录
- **When** 调用方请求 `GET /api/v1/users?page=1&page_size=20`
- **Then** 系统返回 HTTP 200
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `0`，`message` 为 `ok`
- **Then** `data.items` MUST 为用户资料数组
- **Then** 每个用户资料 MUST 包含 `id`、`nickname`、`email`、`status`、`created_at`、`updated_at`
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

用户列表接口 MUST 支持明确白名单过滤参数。系统 MUST 支持 `nickname`、`email`、`status` 过滤，并忽略未定义的 query 参数。所有 `status` 查询参数 MUST 使用共享 enum 校验规则。

#### Scenario: Filter users by nickname
- **Given** 数据库中存在不同昵称的未软删除用户
- **When** 调用方请求 `GET /api/v1/users?nickname=Ali`
- **Then** 系统 MUST 返回 `nickname` 包含 `Ali` 的未软删除用户
- **Then** `data.pagination.total` MUST 反映昵称过滤后的未软删除记录总数

#### Scenario: Filter users by email
- **Given** 数据库中存在邮箱为 `alice@example.com` 的未软删除用户
- **When** 调用方请求 `GET /api/v1/users?email=ALICE@example.com`
- **Then** 系统 MUST 以小写规范化后的邮箱进行精确匹配
- **Then** 返回结果 MUST 只包含邮箱为 `alice@example.com` 且未软删除的用户

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

### Requirement: Document user list API in Swagger

系统 MUST 为 `GET /api/v1/users` 用户列表接口提供与运行时行为一致的 Swagger/OpenAPI 注解和文档输出。

#### Scenario: List endpoint appears in Swagger docs
- **Given** Swagger 文档已生成
- **When** 调用方查看用户接口分组
- **Then** 文档 MUST 包含 `GET /users` 用户列表接口
- **Then** 文档 MUST 描述 `page`、`page_size`、`nickname`、`email`、`status` query 参数
- **Then** 文档 MUST 描述 `status` 允许值为 `100`、`200`、`300`
- **Then** 文档 MUST 描述 HTTP 200 成功响应为统一响应信封包装的分页用户资料
- **Then** 文档 MUST 不得把 `name`、`active`、`password`、`password_hash` 或 `deleted_at` 描述为响应字段
