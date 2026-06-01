## ADDED Requirements

### Requirement: Handle nil errors safely in failure helpers

系统 MUST 保证失败响应 helper 在收到 nil error 时不会 panic，并返回统一失败信封。

#### Scenario: Fail helper receives nil error
- **Given** controller 或 middleware 调用失败响应 helper 时传入 nil error
- **When** 系统将错误转换为响应
- **Then** 系统不得 panic
- **Then** 系统必须返回 HTTP 500
- **Then** 响应 JSON 必须包含 `success: false`、内部错误业务码 `90000` 和对外安全消息 `internal server error`

### Requirement: Keep response contract constants centralized

系统 MUST 在 `common/response` 中集中维护标准成功消息、内部错误消息和业务码常量，避免在响应构造函数中重复硬编码同一契约值。

#### Scenario: Success messages use response constants
- **Given** controller 调用 `response.OK` 或 `response.Created`
- **When** 系统构造成功响应信封
- **Then** `OK` 响应必须使用统一的 `ok` 消息
- **Then** `Created` 响应必须使用统一的 `created` 消息

#### Scenario: Internal error message uses response constant
- **Given** 系统包装非预期错误或 nil error
- **When** 系统构造内部错误失败信封
- **Then** 响应必须使用统一的 `internal server error` 对外安全消息
