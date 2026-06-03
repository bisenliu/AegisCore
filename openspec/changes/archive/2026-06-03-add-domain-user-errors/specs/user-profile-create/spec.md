## MODIFIED Requirements

### Requirement: Enforce unique user identity

系统必须以用户名作为创建用户的唯一业务身份。若用户名已存在，系统必须返回统一冲突错误，不得创建重复用户。repository MUST 将创建时发生的 Ent 唯一约束错误转换为用户领域 `ErrUserAlreadyExists`，service MUST 将该领域错误映射为现有 conflict 应用错误。

#### Scenario: Reject existing user before create
- **Given** 数据库中已存在用户名为 `alice` 的用户
- **When** 调用方请求创建相同用户名的用户
- **Then** service 必须识别用户已存在
- **Then** 系统返回 HTTP 409
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `40000`
- **Then** 响应信封的 `message` 使用 `user-services/internal/errmsg` 中维护的用户已存在文案

#### Scenario: Convert database uniqueness violation to conflict
- **Given** 并发创建导致 `username` 或 `user_id` 唯一索引在数据库写入时冲突
- **When** repository 收到 Ent 唯一约束错误
- **Then** repository 必须将错误转换为用户领域 `ErrUserAlreadyExists`
- **Then** service 必须将 `ErrUserAlreadyExists` 映射为 conflict 应用错误
- **Then** 系统返回 HTTP 409 和统一失败响应信封
- **Then** 响应信封的 `success` 为 `false`，`code` 为 `40000`，`message` 为 `用户已存在`
