## ADDED Requirements

### Requirement: 强制改密登录响应码

系统 MUST 在登录强制改密分支使用 `CodePasswordChangeRequired` 表达受限认证状态。客户端 MUST 能通过响应 envelope 的 code 识别需要进入改密流程，且 MUST NOT 依赖 `password_change_required` 响应字段作为唯一判定依据。

#### Scenario: 强制改密登录返回专用 code

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 登录响应 MUST 返回 HTTP `200 OK`
- **AND** 登录响应 envelope MUST 携带 `CodePasswordChangeRequired`
- **AND** 登录响应 envelope 的 `success` MUST 为 `false`
- **AND** 登录响应 MUST NOT 使用 `CodeOK` 表达该分支
- **AND** 登录响应 MUST 携带 subject 为 `password_change` 的受限 token 数据
- **AND** 登录响应 MUST NOT 携带 refresh token

#### Scenario: 普通登录仍返回成功 code

- **WHEN** 用户凭据有效且账号状态允许普通登录
- **THEN** 登录响应 envelope MUST 携带 `CodeOK`
- **AND** 登录响应 MUST 携带 access token 与 refresh token
- **AND** 登录响应 MUST NOT 携带 `CodePasswordChangeRequired`

#### Scenario: 强制改密分支不创建普通会话

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 系统 MUST 只签发 subject 为 `password_change` 的受限 token
- **AND** 系统 MUST NOT 创建普通 refresh session
- **AND** 系统 MUST NOT 签发 refresh token
