## MODIFIED Requirements

### Requirement: Store authentication session data in Redis as cache and session layer
系统 SHALL 使用 Redis 保存用户 `token_version` 缓存、Refresh Token 会话记录和用户活跃会话索引。Redis 中的 `token_version` 只能作为缓存，缓存未命中或被删除时系统 MUST 使用外部 `user_id` 回源 PostgreSQL 获取真实值。认证会话 Redis key MUST 使用 `config.App.Name` 作为前缀来源；系统 MUST NOT 校验 `app.name`，MUST NOT 为 `app.name` 设置代码级默认值。当 `config.App.Name` 去除首尾空白后非空时，token version 缓存 key MUST 为 `<app.name>:auth:user:<user_id>:token_version`，Refresh Token 会话记录 key MUST 为 `<app.name>:auth:session:<session_id>`，用户活跃会话索引 key MUST 为 `<app.name>:auth:user:<user_id>:sessions`。当 `config.App.Name` 去除首尾空白后为空时，Redis key MUST 保持无前缀业务格式：`auth:user:<user_id>:token_version`、`auth:session:<session_id>` 和 `auth:user:<user_id>:sessions`。用户活跃会话索引 MUST 使用 Redis ZSet，member MUST 使用 `session_id`，score MUST 使用该会话过期时间的 Unix 时间戳。系统 MUST 在写入、读取或删除该用户活跃会话索引时，按当前 Unix 时间戳执行过期 member 清理。

#### Scenario: Token version cache miss reads PostgreSQL
- **Given** Redis 中不存在某用户的 token version 缓存
- **When** 系统需要校验该用户的 token version
- **Then** 系统 MUST 使用外部 `user_id` 查询 PostgreSQL 获取真实 `token_version`
- **Then** 系统 MUST 将真实版本回填 Redis 缓存

#### Scenario: User security update deletes Redis cache after PostgreSQL update
- **Given** 用户级安全事件需要整体失效 token
- **When** 系统处理该安全事件
- **Then** 系统 MUST 先更新 PostgreSQL 中的 `token_version`
- **Then** 系统 MUST 再删除或刷新 Redis token version 缓存
- **Then** 系统 MUST NOT 只更新 Redis 而不更新 PostgreSQL

#### Scenario: Session creation indexes expiration timestamp
- **Given** 登录或 Refresh Token 轮转需要创建新的 Redis 会话记录
- **Given** `config.App.Name` 去除首尾空白后为 `aegiscore-user-services`
- **When** 系统保存会话记录和用户活跃会话索引
- **Then** 系统 MUST 将 `aegiscore-user-services:auth:session:<session_id>` 保存为带 TTL 的会话记录
- **Then** 系统 MUST 将 `session_id` 写入 `aegiscore-user-services:auth:user:<user_id>:sessions` ZSet
- **Then** 该 ZSet member 的 score MUST 等于该会话过期时间的 Unix 时间戳
- **Then** 系统 MUST 在写入索引时清理该 ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member

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
