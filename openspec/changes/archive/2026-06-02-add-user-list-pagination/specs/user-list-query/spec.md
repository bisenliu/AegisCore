## ADDED Requirements

### Requirement: List users with pagination

系统 MUST 提供 `GET /api/v1/users` 用户列表接口。接口 MUST 支持分页查询，MUST 通过统一响应信封返回分页用户资料列表，且返回用户资料 MUST 不包含 `password`。

#### Scenario: List users with explicit pagination
- **Given** 数据库中存在多个用户记录
- **When** 调用方请求 `GET /api/v1/users?page=1&page_size=20`
- **Then** 系统返回 HTTP 200
- **Then** 响应信封的 `success` 为 `true`，`code` 为 `0`，`message` 为 `ok`
- **Then** `data.items` MUST 为用户资料数组
- **Then** 每个用户资料 MUST 包含 `id`、`name`、`email`、`active`、`created_at`、`updated_at`
- **Then** 每个用户资料 MUST NOT 包含 `password`
- **Then** `data.pagination.page` MUST 为 `1`
- **Then** `data.pagination.page_size` MUST 为 `20`
- **Then** `data.pagination.total` MUST 为过滤后的总记录数
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
- **Given** 数据库中没有匹配当前过滤条件的用户记录
- **When** 调用方请求用户列表接口
- **Then** 系统返回 HTTP 200
- **Then** `data.items` MUST 为空数组
- **Then** `data.pagination.total` MUST 为 `0`
- **Then** `data.pagination.total_pages` MUST 为 `0`

### Requirement: Filter listed users

用户列表接口 MUST 支持明确白名单过滤参数。系统 MUST 支持 `name`、`email`、`active` 过滤，并忽略未定义的 query 参数。

#### Scenario: Filter users by name
- **Given** 数据库中存在不同名称的用户
- **When** 调用方请求 `GET /api/v1/users?name=Ali`
- **Then** 系统 MUST 返回名称包含 `Ali` 的用户
- **Then** `data.pagination.total` MUST 反映名称过滤后的总记录数

#### Scenario: Filter users by email
- **Given** 数据库中存在邮箱为 `alice@example.com` 的用户
- **When** 调用方请求 `GET /api/v1/users?email=ALICE@example.com`
- **Then** 系统 MUST 以小写规范化后的邮箱进行精确匹配
- **Then** 返回结果 MUST 只包含邮箱为 `alice@example.com` 的用户

#### Scenario: Filter users by active status
- **Given** 数据库中同时存在启用和停用用户
- **When** 调用方请求 `GET /api/v1/users?active=true`
- **Then** 系统 MUST 只返回 `active` 为 `true` 的用户

#### Scenario: Reject invalid active filter
- **Given** 调用方提供无法解析为布尔值的 `active` 参数
- **When** 调用方请求用户列表接口
- **Then** 系统 MUST 返回 HTTP 400
- **Then** 响应 MUST 使用统一失败信封

### Requirement: Preserve layered user list behavior

用户列表实现 MUST 保持 controller/service/repository 分层职责。HTTP query 解析 MUST 位于 controller，过滤条件清洗和分页业务编排 MUST 位于 service，Ent 查询与 count MUST 位于 repository。

#### Scenario: Repository returns unexpected list error
- **Given** 数据库执行用户列表查询或 count 时发生非预期错误
- **When** service 处理 repository 返回的错误
- **Then** 错误 MUST 被转换为内部错误
- **Then** API 响应 MUST 使用内部错误业务码 `90000` 和 `internal server error`
- **Then** 响应 MUST NOT 暴露底层数据库细节

### Requirement: Document user list API in Swagger

系统 MUST 为 `GET /api/v1/users` 用户列表接口提供与运行时行为一致的 Swagger/OpenAPI 注解和文档输出。

#### Scenario: List endpoint appears in Swagger docs
- **Given** Swagger 文档已生成
- **When** 调用方查看用户接口分组
- **Then** 文档 MUST 包含 `GET /users` 用户列表接口
- **Then** 文档 MUST 描述 `page`、`page_size`、`name`、`email`、`active` query 参数
- **Then** 文档 MUST 描述 HTTP 200 成功响应为统一响应信封包装的分页用户资料
- **Then** 文档 MUST 不得把 `password` 描述为响应字段
