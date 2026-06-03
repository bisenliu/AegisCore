## ADDED Requirements

### Requirement: Separate credential authentication from login token issuance

系统 SHALL 在登录流程中保持用户凭据认证与认证成功后的 token 签发策略分离。凭据认证 MUST 校验归一化后的 `username` 和明文密码均非空，MUST 按 `username` 读取未软删除用户认证资料，MUST 使用共享密码校验 helper 验证 `password_hash`，并 MUST 将用户不存在、密码不匹配、密码 hash 校验错误和禁用用户状态映射为统一凭据无效响应。`status=300` 用户在密码校验通过后 MUST 被视为认证成功，并由登录签发策略返回受限改密凭据。

#### Scenario: Credential authentication rejects empty login input
- **Given** 调用方提交空白 `username` 或空白 `password`
- **When** 系统执行登录凭据认证
- **Then** 系统 MUST 返回统一凭据无效响应
- **Then** 系统 MUST NOT 查询用户资料或签发任何 token

#### Scenario: Credential authentication hides invalid credential details
- **Given** 用户不存在、已软删除、密码校验失败或密码 hash 格式非法
- **When** 调用方提交登录请求
- **Then** 系统 MUST 返回统一凭据无效响应
- **Then** 响应和业务日志 MUST NOT 包含明文密码、完整 hash、salt 或 hash 参数

#### Scenario: Must-change-password remains an issuance policy
- **Given** 用户存在且 `status` 为 `300`
- **Given** 用户提交的密码与 PostgreSQL 中的 `password_hash` 校验通过
- **When** 调用方提交登录请求
- **Then** 系统 MUST 将该用户视为凭据认证成功
- **Then** 登录签发策略 MUST 返回受限改密凭据
- **Then** 系统 MUST NOT 创建普通 Redis Refresh Token 会话

#### Scenario: Disabled user remains unauthenticated
- **Given** 用户存在且 `status` 为禁用状态
- **Given** 用户提交的密码与 PostgreSQL 中的 `password_hash` 校验通过
- **When** 调用方提交登录请求
- **Then** 系统 MUST 返回统一凭据无效响应
- **Then** 系统 MUST NOT 签发普通 token 或受限改密凭据

### Requirement: Verify password-change credentials before loading user state

系统 SHALL 在修改密码流程中先验证受限改密凭据并解析外部用户 UUID，再读取用户状态和更新凭证。受限改密凭据验证 MUST 支持剥离可选 `Bearer ` 前缀，MUST 解析 password-change token，MUST 校验服务端当前 `token_version` 与 token claims 一致，并 MUST 将 claims 中的 `user_id` 解析为 UUID。用户存在性检查、用户仍处于 `status=300` 的校验、新密码 hash、凭证更新和 Redis token version 缓存失效 MUST 继续由修改密码业务流程负责。

#### Scenario: Password-change token validation rejects invalid token
- **Given** 调用方提交空白、格式非法、签名无效或 subject 非改密凭据的 token
- **When** 系统验证受限改密凭据
- **Then** 系统 MUST 返回 token 无效响应
- **Then** 系统 MUST NOT 更新用户凭证

#### Scenario: Password-change token validation rejects changed token version
- **Given** 受限改密凭据签名有效且未过期
- **Given** token claims 中的 `token_version` 与服务端当前版本不一致
- **When** 调用方请求修改密码
- **Then** 系统 MUST 返回 token 无效响应
- **Then** 系统 MUST NOT 查询后续状态或更新用户凭证

#### Scenario: Password-change flow owns user state validation
- **Given** 受限改密凭据通过 token 校验并解析出 UUID `user_id`
- **When** 系统继续处理修改密码请求
- **Then** 修改密码流程 MUST 使用该 UUID 查询用户
- **Then** 修改密码流程 MUST 校验用户仍处于 `status=300`
- **Then** 只有状态校验通过后系统 MUST 更新 `password_hash`、将状态更新为 `100` 并失效 token version 缓存
