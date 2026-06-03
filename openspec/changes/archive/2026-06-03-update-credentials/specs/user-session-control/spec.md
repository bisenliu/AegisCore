## ADDED Requirements

### Requirement: Update user credentials through a credential update contract

系统 SHALL 在修改密码成功后通过用户仓储的通用凭证更新契约持久化新凭证。凭证更新 MUST 面向未软删除用户，MUST 写入新的 `password_hash`，MUST 更新目标用户状态，MUST 递增 PostgreSQL 中的 `token_version`，并 MUST 返回更新后的 `token_version`。系统 MUST NOT 在该流程中读取或写入旧 `password` 字段。

#### Scenario: Password change updates credentials and invalidates tokens
- **Given** 用户存在且 `deleted_at` 为 `NULL`
- **Given** 用户当前状态为 `300`
- **Given** 调用方持有有效的受限改密凭据
- **When** 调用方提交有效的新密码并完成密码哈希
- **Then** 系统 MUST 通过凭证更新契约写入新的 `password_hash`
- **Then** 系统 MUST 将用户状态更新为 `100`
- **Then** 系统 MUST 递增 PostgreSQL 中的 `token_version`
- **Then** 系统 MUST 删除或刷新 Redis token version 缓存，使旧凭据不可继续用于改密

#### Scenario: Credential update ignores soft deleted users
- **Given** PostgreSQL 中对应用户的 `deleted_at` 不为 `NULL`
- **When** 系统尝试更新该用户凭证
- **Then** 系统 MUST 将该用户视为不存在
- **Then** 系统 MUST NOT 更新该用户的 `password_hash`
- **Then** 系统 MUST NOT 更新该用户状态或递增该用户 `token_version`

#### Scenario: Credential update returns current token version after persistence
- **Given** 用户凭证更新成功
- **When** repository 完成 PostgreSQL 更新
- **Then** repository MUST 返回更新后的 `token_version`
- **Then** 调用方 MUST NOT 只依赖 Redis token version 缓存判断该次安全更新是否完成
