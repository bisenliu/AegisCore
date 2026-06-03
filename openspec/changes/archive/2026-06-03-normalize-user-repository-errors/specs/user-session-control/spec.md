## ADDED Requirements

### Requirement: User session repository returns domain user-not-found errors

用户会话控制能力 SHALL 保持 service/repository 分层边界。PostgreSQL 用户 repository 在会话控制流程中读取 `token_version`、递增 `token_version` 或更新用户凭证时，若目标未软删除用户不存在，MUST 返回 `domain.ErrUserNotFound`，MUST NOT 直接构造 `common/response` 应用错误。service 层 MUST 负责将该领域错误映射为现有认证失败、token 无效、not found 或内部错误响应语义。

#### Scenario: Token version lookup misses user
- **Given** Redis token version 缓存未命中
- **Given** PostgreSQL 中不存在对应未软删除用户
- **When** 用户 repository 读取该用户 `token_version`
- **Then** PostgreSQL repository MUST 返回 `domain.ErrUserNotFound`
- **Then** PostgreSQL repository MUST NOT 返回 `response.NotFoundError` 或其他 `common/response` 应用错误
- **Then** service 层 MUST 继续按现有认证或 token 校验流程映射该错误

#### Scenario: Logout all devices misses user during token version increment
- **Given** 请求已通过 Access Token 认证
- **Given** PostgreSQL 中不存在对应未软删除用户
- **When** 用户 repository 尝试递增该用户 `token_version`
- **Then** PostgreSQL repository MUST 返回 `domain.ErrUserNotFound`
- **Then** PostgreSQL repository MUST NOT 构造 HTTP not found 应用错误
- **Then** service 层 MUST 负责输出与迁移前兼容的失败响应

#### Scenario: Credential update misses user
- **Given** 修改密码流程已验证受限改密凭据
- **Given** PostgreSQL 中不存在对应未软删除用户
- **When** 用户 repository 尝试更新该用户 `password_hash`、状态和 `token_version`
- **Then** PostgreSQL repository MUST 返回 `domain.ErrUserNotFound`
- **Then** PostgreSQL repository MUST NOT 更新任何软删除用户或不存在用户
- **Then** service 层 MUST 负责将该领域错误映射为现有修改密码失败语义

#### Scenario: Unexpected database error remains internal
- **Given** PostgreSQL 在读取 token version、递增 token version 或更新凭据时发生非 not found 错误
- **When** 用户 repository 返回错误给 service 层
- **Then** repository MUST NOT 将该错误伪装为 `domain.ErrUserNotFound`
- **Then** service 层 MUST 继续将非预期错误映射为内部错误或既有安全失败语义
