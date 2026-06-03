## ADDED Requirements

### Requirement: Return external user identity fields in user profile data
系统 SHALL 保持用户相关 API 的 `common/response.Envelope` 外层响应契约不变，同时用户资料 `data` MUST 只公开外部用户身份字段和非敏感资料字段。用户资料响应 MUST 返回 `user_id` 和 `username`，MUST NOT 返回内部数据库 `id`、`email`、`password` 或密码哈希。

#### Scenario: Create user response exposes external identity
- **Given** 用户创建成功
- **When** controller 输出成功响应信封
- **Then** 响应信封 MUST 保持 `success`、`code`、`message`、`data` 结构
- **Then** `data` MUST 包含 `user_id` 和 `username`
- **Then** `data` MUST NOT 包含 `id`、`email`、`password` 或密码哈希

#### Scenario: Query user response exposes external identity
- **Given** 用户资料查询成功
- **When** controller 输出成功响应信封
- **Then** 响应信封 MUST 保持 `success`、`code`、`message`、`data` 结构
- **Then** `data` MUST 包含 `user_id` 和 `username`
- **Then** `data` MUST NOT 包含 `id`、`email`、`password` 或密码哈希

#### Scenario: User errors preserve envelope shape
- **Given** 用户创建、查询或登录请求失败
- **When** 系统返回错误响应
- **Then** 响应 MUST 继续使用统一失败响应信封
- **Then** 响应 MUST NOT 通过错误消息泄露内部数据库 `id`、密码明文、完整密码 hash 或底层数据库细节
