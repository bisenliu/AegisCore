## Why

`common/security/auth` 的 JWT 解析当前在 `user_id` claim 存在但不是 UUID 时返回 `ErrMissingUserID`，把“字段存在但格式非法”误判为“字段缺失”。这会误导认证问题排查、日志分析和调用方错误分类，也弱化了共享 JWT 凭证原语的错误语义。

## What Changes

- 在 `common/security/auth` 中新增 `ErrInvalidUserID`，用于表达 JWT `user_id` 或签发输入中的用户 ID 存在但不是合法 UUID。
- 调整 JWT token 解析逻辑：`user_id` 为空仍返回 `ErrMissingUserID`，`uuid.Parse(user_id)` 失败时返回 `ErrInvalidUserID`。
- 通过单元测试覆盖缺失 `user_id` 与非法 UUID `user_id` 的不同错误语义。
- 不改变 JWT claim 结构、subject、签名算法、issuer/audience 校验、HTTP 401 认证失败响应或认证业务码。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `common-credentials`: JWT token 凭证原语需要区分缺失 `user_id` 与格式非法的 `user_id`，并暴露准确的错误常量。

## Impact

- 影响代码：`common/security/auth/jwt.go` 及其测试。
- 影响 capability：`common-credentials`；`user-authentication` 继续消费 shared JWT 服务，但对外 HTTP 响应兼容。
- API 兼容性：HTTP API 响应状态码、响应信封和业务错误码不变；Go 包会新增错误常量，已有 `ErrMissingUserID` 对空值语义保持不变。
- 数据与配置：不涉及数据库 schema、migration、Redis key 或运行时配置变更。
