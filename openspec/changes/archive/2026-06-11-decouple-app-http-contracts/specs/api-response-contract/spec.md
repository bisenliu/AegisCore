## MODIFIED Requirements

### Requirement: Return paginated success responses

系统 MUST 在 `common/contract/response` 中提供可复用分页响应数据结构。分页列表成功响应 MUST 保持统一 `Envelope` 顶层字段，并将列表数据包装到 `data.items`，分页元信息包装到 `data.pagination`。HTTP transport 层 MUST 负责将 app 层返回的列表应用结果映射为分页响应数据结构；app service MUST NOT 返回 `response.PaginatedData` 或 `response.Pagination`。

#### Scenario: Return paginated list payload
- **Given** controller 成功处理分页列表请求并获得列表数据和分页元信息
- **When** controller 使用 `response.OK` 返回分页数据
- **Then** 系统返回 HTTP 200
- **Then** 响应 JSON 包含 `success: true`、`code: 0`、`message: ok`
- **Then** `data.items` MUST 包含当前页业务数据数组
- **Then** `data.pagination` MUST 包含 `page`、`page_size`、`total`、`total_pages`

#### Scenario: Return empty paginated list
- **Given** 分页列表请求没有匹配到任何记录
- **When** controller 返回分页数据
- **Then** 系统返回 HTTP 200
- **Then** `data.items` MUST 为空数组
- **Then** `data.pagination.total` MUST 为 `0`
- **Then** `data.pagination.total_pages` MUST 为 `0`

#### Scenario: Map app list result to paginated response at transport boundary
- **Given** app service 返回列表应用结果、当前页参数和总数
- **When** HTTP controller 准备返回列表响应
- **Then** controller 或 feature-local HTTP mapper MUST 使用 `response.NewPagination` 和 `response.NewPaginatedData` 构造分页响应数据
- **Then** app service MUST NOT 导入 `common/contract/response` 以构造 HTTP 分页响应契约

### Requirement: Return failure responses with application errors

系统必须将应用错误转换为统一失败信封，并使用错误对象中的 HTTP 状态码、数字业务码和消息。参数校验失败响应必须在统一失败信封中携带字段级 `errors` 明细。HTTP transport 层、中间件或 `common/contract/response` helper MUST 负责构造 HTTP 应用错误；app service MUST 以领域错误、应用错误分类或普通 Go error 表达失败原因，并由 controller 映射为统一失败信封。

#### Scenario: Return bad request
- **Given** controller 检测到请求格式错误、请求体格式错误或参数无法解析
- **When** controller 调用 `response.BadRequest` 或返回 `response.BadRequestError`
- **Then** 系统返回 HTTP 400
- **Then** 响应 JSON 包含 `success: false`、`code: 10000` 和调用方可读消息
- **Then** 响应不包含 `data`

#### Scenario: Return validation failed
- **Given** 请求参数绑定成功但 required、min、max、len、email、enum、gt、gte、lt 或 lte 等校验规则不通过
- **When** validation helper 生成失败响应
- **Then** 系统返回 HTTP 400
- **Then** 响应 JSON 包含 `success: false`、`code: 10001` 和顶层消息 `请求参数验证失败`
- **Then** 响应 JSON 包含 `errors` 数组
- **Then** 每个 `errors` 元素必须包含 `field`、`label`、`rule`、`message`
- **Then** 响应不包含业务 `data`

#### Scenario: Return unauthenticated
- **Given** 请求缺少有效认证信息
- **When** controller 或中间件调用 `response.Unauthenticated` 或返回 `response.UnauthenticatedError`
- **Then** 系统返回 HTTP 401
- **Then** 响应 JSON 包含 `success: false`、`code: 20000` 和调用方可读消息

#### Scenario: Return forbidden
- **Given** 已认证调用方无权访问资源或执行操作
- **When** controller 或中间件将失败原因映射为 `response.ForbiddenError`
- **Then** 系统返回 HTTP 403
- **Then** 响应 JSON 包含 `success: false`、`code: 30000` 和调用方可读消息

#### Scenario: Return conflict
- **Given** app service 返回业务冲突或资源状态不允许当前操作的领域错误或应用错误分类
- **When** controller 将该失败原因映射为 `response.ConflictError`
- **Then** 系统返回 HTTP 409
- **Then** 响应 JSON 包含 `success: false`、`code: 40000` 和调用方可读消息

#### Scenario: Return not found
- **Given** repository 或 app service 返回 not found 领域错误或应用错误分类
- **When** controller 将该失败原因映射为 `response.NotFoundError` 并调用 `response.Fail`
- **Then** 系统返回 HTTP 404
- **Then** 响应 JSON 包含 `success: false`、`code: 50000` 和对应消息

#### Scenario: Wrap unexpected error
- **Given** app service 或 repository 返回普通 Go error
- **When** controller 或 response helper 调用 `response.FromError`
- **Then** 错误被包装为内部错误
- **Then** HTTP 状态码为 500
- **Then** 响应 JSON 包含 `success: false`、`code: 90000` 和对外安全消息 `internal server error`

#### Scenario: Keep HTTP error construction out of app services
- **Given** 开发者修改用户资料或认证会话 app service、ports、credential、token 或 session 组件
- **When** 这些组件需要返回失败原因
- **Then** 这些组件 MUST NOT 构造 `response.BadRequestError`、`response.UnauthenticatedError`、`response.TokenInvalidError`、`response.TokenExpiredError`、`response.ForbiddenError`、`response.ConflictError`、`response.NotFoundError` 或 `response.FromError`
- **Then** HTTP controller、HTTP middleware 或 feature-local HTTP mapper MUST 将失败原因转换为统一失败信封
