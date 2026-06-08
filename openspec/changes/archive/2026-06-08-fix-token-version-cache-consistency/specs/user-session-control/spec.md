## MODIFIED Requirements

### Requirement: Logout all devices through token version increment
系统 SHALL 支持退出全部设备。退出全部设备时，系统 MUST 通过认证会话组件在 PostgreSQL 中原子递增该用户 `token_version`，更新成功后将 Redis 中该用户的 token version cache 覆盖为递增后的新版本，并删除全部活跃会话记录和会话索引。`AuthService` MUST 仅从认证上下文提取当前用户身份并调用认证会话组件，不得直接执行用户 repository 写入或 Redis 会话清理。

#### Scenario: Logout all sessions invalidates tokens
- **Given** 请求已通过 Access Token 认证
- **When** 调用方请求退出全部设备
- **Then** 系统 MUST 先在 PostgreSQL 中递增该用户 `token_version`
- **Then** 系统 MUST 将 Redis 中该用户的 token version cache 写入为递增后的新版本
- **Then** 系统 MUST 删除该用户所有 Redis Refresh Token 会话记录
- **Then** 系统 MUST 清空该用户活跃会话索引
- **Then** 旧 Access Token MUST 因版本不一致而失效

#### Scenario: Auth session manager owns logout all writes
- **Given** 请求已通过 Access Token 认证
- **When** `AuthService` 处理退出全部设备流程
- **Then** `AuthService` MUST 从认证上下文提取并校验当前 `user_id`
- **Then** `AuthService` MUST 调用认证会话组件执行全部会话吊销
- **Then** 认证会话组件 MUST 调用用户 repository 原子递增 `token_version`
- **Then** 认证会话组件 MUST 调用 auth session repository 写入新 token version cache 并清理全部 Refresh Token 会话记录
- **Then** `AuthService` MUST NOT 直接持有用户 repository 来完成退出全部设备写操作

#### Scenario: Logout all fails when token version cache cannot be refreshed
- **Given** PostgreSQL 已成功递增该用户 `token_version`
- **Given** Redis 中该用户可能仍存在旧 token version cache
- **When** 系统无法将 Redis token version cache 写入为递增后的新版本
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 将该次退出全部设备报告为成功

### Requirement: Store authentication session data in Redis as cache and session layer
系统 SHALL 使用 Redis 保存用户 `token_version` 缓存、Refresh Token 会话记录和用户活跃会话索引。Redis 中的 `token_version` 只能作为缓存，缓存未命中或被删除时系统 MUST 由认证会话 service 组件或 token version resolver 使用外部 `user_id` 回源 PostgreSQL 获取真实值。用户级安全事件递增 PostgreSQL `token_version` 后，系统 MUST 将 Redis token version cache 覆盖为递增后的新版本，不得只删除旧缓存作为成功路径。认证会话 Redis key MUST 使用 `config.App.Name` 作为前缀来源；系统 MUST NOT 校验 `app.name`，MUST NOT 为 `app.name` 设置代码级默认值。当 `config.App.Name` 去除首尾空白后非空时，token version 缓存 key MUST 为 `<app.name>:auth:user:<user_id>:token_version`，Refresh Token 会话记录 key MUST 为 `<app.name>:auth:session:<session_id>`，用户活跃会话索引 key MUST 为 `<app.name>:auth:user:<user_id>:sessions`。当 `config.App.Name` 去除首尾空白后为空时，Redis key MUST 保持无前缀业务格式：`auth:user:<user_id>:token_version`、`auth:session:<session_id>` 和 `auth:user:<user_id>:sessions`。用户活跃会话索引 MUST 使用 Redis ZSet，member MUST 使用 `session_id`，score MUST 使用该会话过期时间的 Unix 时间戳。系统 MUST 以 Redis session key 的实际 TTL 推导会话过期时间，并使会话 payload 中的 `ExpiresAt`、session key TTL 和用户活跃会话索引 score 保持一致。系统 MUST 在写入、读取或删除该用户活跃会话索引时，按当前 Unix 时间戳执行过期 member 清理。系统 MUST 为用户活跃会话索引设计过期或清理策略，避免没有活跃会话的 ZSet key 和已过期 `session_id` 长期残留。

#### Scenario: Token version cache miss reads PostgreSQL
- **Given** Redis 中不存在某用户的 token version 缓存
- **When** 系统需要校验该用户的 token version
- **Then** 系统 MUST 使用外部 `user_id` 查询 PostgreSQL 获取真实 `token_version`
- **Then** 系统 MUST 将真实版本回填 Redis 缓存

#### Scenario: User security update refreshes Redis cache after PostgreSQL update
- **Given** 用户级安全事件需要整体失效 token
- **When** 系统处理该安全事件
- **Then** 系统 MUST 先更新 PostgreSQL 中的 `token_version`
- **Then** 系统 MUST 再将 Redis token version cache 写入为 PostgreSQL 返回的新版本
- **Then** 系统 MUST NOT 只更新 Redis 而不更新 PostgreSQL

#### Scenario: Token version cache refresh failure is not successful revocation
- **Given** 用户级安全事件已更新 PostgreSQL 中的 `token_version`
- **When** Redis token version cache 无法写入 PostgreSQL 返回的新版本
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 在旧 Redis cache 仍可能被认证路径信任时报告吊销成功

#### Scenario: Session creation indexes expiration timestamp
- **Given** 登录或 Refresh Token 轮转需要创建新的 Redis 会话记录
- **Given** `config.App.Name` 去除首尾空白后为 `aegiscore-user-services`
- **When** 系统保存会话记录和用户活跃会话索引
- **Then** 系统 MUST 将 `aegiscore-user-services:auth:session:<session_id>` 保存为带 TTL 的会话记录
- **Then** 系统 MUST 将 `session_id` 写入 `aegiscore-user-services:auth:user:<user_id>:sessions` ZSet
- **Then** 该 ZSet member 的 score MUST 等于该会话过期时间的 Unix 时间戳
- **Then** 系统 MUST 在写入索引时清理该 ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member

#### Scenario: Session creation uses one expiration source
- **Given** 登录或 Refresh Token 轮转需要创建新的 Redis 会话记录
- **Given** 调用方传入的会话 payload 包含空白或非空白 `ExpiresAt`
- **When** 系统根据有效 Refresh Token TTL 保存会话记录和用户活跃会话索引
- **Then** 系统 MUST 以该 TTL 和当前时间推导唯一会话过期时间
- **Then** Redis session key 的 TTL MUST 与该推导过期时间一致
- **Then** 序列化会话 payload 中的 `ExpiresAt` MUST 与该推导过期时间一致
- **Then** 用户活跃会话 ZSet member 的 score MUST 与该推导过期时间一致
- **Then** 系统 MUST NOT 让调用方传入的旧 `ExpiresAt` 导致 ZSet score 与 session key 实际过期时间不一致

#### Scenario: User session index receives expiration or bounded cleanup
- **Given** 系统创建或轮转 Refresh Token 会话
- **When** 系统写入用户活跃会话 ZSet 索引
- **Then** 系统 MUST 为该 ZSet key 设置可使无活跃会话索引最终消失的过期策略，或提供等价的有界清理策略
- **Then** 该策略 MUST NOT 在仍存在未过期会话时提前删除用户活跃会话索引
- **Then** 过期 `session_id` MUST NOT 只能依赖未来批量退出操作才被清理

#### Scenario: Empty app name keeps unprefixed Redis keys
- **Given** `config.App.Name` 去除首尾空白后为空
- **When** 系统保存 token version 缓存、Refresh Token 会话记录或用户活跃会话索引
- **Then** 系统 MUST 使用 `auth:user:<user_id>:token_version` 作为 token version 缓存 key
- **Then** 系统 MUST 使用 `auth:session:<session_id>` 作为 Refresh Token 会话记录 key
- **Then** 系统 MUST 使用 `auth:user:<user_id>:sessions` 作为用户活跃会话索引 key
- **Then** 系统 MUST NOT 使用代码级默认服务名补齐 Redis key 前缀

#### Scenario: Logout current session removes indexed member and stale members
- **Given** 请求已通过 Access Token 认证
- **Given** 认证上下文包含当前 `user_id` 和 `session_id`
- **Given** `config.App.Name` 去除首尾空白后为 `aegiscore-user-services`
- **When** 调用方请求退出当前设备
- **Then** 系统 MUST 删除当前 `session_id` 对应的 Redis 会话记录
- **Then** 系统 MUST 从 `aegiscore-user-services:auth:user:<user_id>:sessions` ZSet 移除当前 `session_id`
- **Then** 系统 MUST 清理该 ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member

#### Scenario: Logout all sessions reads only non-expired indexed members
- **Given** 请求已通过 Access Token 认证
- **Given** `config.App.Name` 去除首尾空白后为 `aegiscore-user-services`
- **When** 调用方请求退出全部设备
- **Then** 系统 MUST 先清理 `aegiscore-user-services:auth:user:<user_id>:sessions` ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member
- **Then** 系统 MUST 从该 ZSet 读取仍未过期的 `session_id`
- **Then** 系统 MUST 删除读取到的 Redis 会话记录
- **Then** 系统 MUST 删除或清空该用户活跃会话索引

#### Scenario: Stale index data does not inflate future session operations
- **Given** 用户活跃会话 ZSet 中存在已过期 `session_id` member
- **When** 系统创建新会话、删除当前会话或删除该用户全部会话
- **Then** 系统 MUST 在读取或写入业务相关 member 前清理按 score 已过期的 member
- **Then** 系统 MUST 避免长期遍历已过期残留作为活跃会话
- **Then** 后续会话统计、管理或审计能力 MUST 能基于清理后的索引语义区分活跃会话和过期残留

### Requirement: Update user credentials through a credential update contract

系统 SHALL 在修改密码成功后通过用户仓储的通用凭证更新契约持久化新凭证。凭证更新 MUST 面向未软删除用户，MUST 写入新的 `password_hash`，MUST 更新目标用户状态，MUST 递增 PostgreSQL 中的 `token_version`，并 MUST 返回更新后的 `token_version`。系统 MUST NOT 在该流程中读取或写入旧 `password` 字段。修改密码流程完成凭证更新后，系统 MUST 将 Redis token version cache 覆盖为更新后的新版本，使旧凭据和旧认证会话不再因为旧缓存命中而继续通过版本校验。

#### Scenario: Password change updates credentials and invalidates tokens
- **Given** 用户存在且 `deleted_at` 为 `NULL`
- **Given** 用户当前状态为 `300`
- **Given** 调用方持有有效的受限改密凭据
- **When** 调用方提交有效的新密码并完成密码哈希
- **Then** 系统 MUST 通过凭证更新契约写入新的 `password_hash`
- **Then** 系统 MUST 将用户状态更新为 `100`
- **Then** 系统 MUST 递增 PostgreSQL 中的 `token_version`
- **Then** 系统 MUST 将 Redis token version cache 写入为递增后的新版本，使旧凭据不可继续用于改密

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

#### Scenario: Password change fails when token version cache cannot be refreshed
- **Given** 用户凭证更新已成功递增 PostgreSQL `token_version`
- **When** 系统无法将 Redis token version cache 写入为递增后的新版本
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 将该次修改密码报告为成功
