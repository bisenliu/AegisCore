## MODIFIED Requirements

### Requirement: Return failure responses with application errors

系统必须将应用错误转换为统一失败信封，并使用错误对象中的 HTTP 状态码、数字业务码和消息。参数校验失败响应必须在统一失败信封中携带字段级 `errors` 明细。

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
- **When** controller 或 service 返回 `response.ForbiddenError`
- **Then** 系统返回 HTTP 403
- **Then** 响应 JSON 包含 `success: false`、`code: 30000` 和调用方可读消息

#### Scenario: Return conflict
- **Given** service 检测到业务冲突或资源状态不允许当前操作
- **When** service 返回 `response.ConflictError`
- **Then** 系统返回 HTTP 409
- **Then** 响应 JSON 包含 `success: false`、`code: 40000` 和调用方可读消息

#### Scenario: Return not found
- **Given** repository 或 service 返回 not found 应用错误
- **When** controller 调用 `response.Fail`
- **Then** 系统返回 HTTP 404
- **Then** 响应 JSON 包含 `success: false`、`code: 50000` 和对应消息

#### Scenario: Wrap unexpected error
- **Given** service 或 repository 返回普通 Go error
- **When** 系统调用 `response.FromError`
- **Then** 错误被包装为内部错误
- **Then** HTTP 状态码为 500
- **Then** 响应 JSON 包含 `success: false`、`code: 90000` 和对外安全消息 `internal server error`
