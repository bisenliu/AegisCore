## ADDED Requirements

### Requirement: HTTP response helper wrapper 覆盖

系统 MUST 在 `common/http/response` 中为共享 HTTP response helper wrapper 保持直接单元测试覆盖，测试 MUST 锁定当前统一 response envelope、应用错误码、公开 message、HTTP status、`data` 或 `errors` 字段行为，并 MUST NOT 接受旧 envelope、旧错误消息格式、旧 helper alias 或旧 HTTP status 的兼容路径。

#### Scenario: 创建成功响应

- **WHEN** 调用方使用 `Created` 写入创建成功响应
- **THEN** 系统 MUST 返回 `201 Created`
- **AND** 响应 envelope MUST 为 `success=true`、`code=CodeOK`、`message=MessageCreated`
- **AND** 响应 envelope MUST 携带调用方传入的 `data`

#### Scenario: 无内容成功响应

- **WHEN** 调用方使用 `NoContent` 写入无内容成功响应
- **THEN** 系统 MUST 返回 `204 No Content`
- **AND** 响应 body MUST 为空

#### Scenario: 校验失败响应

- **WHEN** 调用方使用 `ValidationFailed` 写入字段语义校验失败响应
- **THEN** 系统 MUST 返回 `400 Bad Request`
- **AND** 响应 envelope MUST 为 `success=false`、`code=CodeValidationFailed`
- **AND** 响应 envelope MUST 使用调用方提供的公开 message

#### Scenario: 未认证响应

- **WHEN** 调用方使用 `Unauthenticated` 写入未认证响应
- **THEN** 系统 MUST 返回 `401 Unauthorized`
- **AND** 响应 envelope MUST 为 `success=false`、`code=CodeUnauthenticated`
- **AND** 响应 envelope MUST 使用调用方提供的公开 message

#### Scenario: 权限不足响应

- **WHEN** 调用方使用 `Forbidden` 写入权限不足响应
- **THEN** 系统 MUST 返回 `403 Forbidden`
- **AND** 响应 envelope MUST 为 `success=false`、`code=CodeForbidden`
- **AND** 响应 envelope MUST 使用调用方提供的公开 message

#### Scenario: 冲突响应

- **WHEN** 调用方使用 `Conflict` 写入领域冲突或资源状态冲突响应
- **THEN** 系统 MUST 返回 `409 Conflict`
- **AND** 响应 envelope MUST 为 `success=false`、`code=CodeConflict`
- **AND** 响应 envelope MUST 使用调用方提供的公开 message

#### Scenario: 未找到响应

- **WHEN** 调用方使用 `NotFound` 写入资源不存在响应
- **THEN** 系统 MUST 返回 `404 Not Found`
- **AND** 响应 envelope MUST 为 `success=false`、`code=CodeNotFound`
- **AND** 响应 envelope MUST 使用调用方提供的公开 message
