## MODIFIED Requirements

### Requirement: Store authentication session data in Redis as cache and session layer
系统 SHALL 使用 Redis 保存用户 `token_version` 缓存、Refresh Token 会话记录和用户活跃会话索引。Redis 中的 `token_version` 只能作为缓存，缓存未命中或被删除时系统 MUST 由认证会话 service 组件或 token version resolver 使用外部 `user_id` 回源 PostgreSQL 获取真实值。用户级安全事件递增 PostgreSQL `token_version` 后，系统 MUST 将 Redis token version cache 覆盖为递增后的新版本，不得只删除旧缓存作为成功路径。认证会话 Redis key MUST 使用 `config.App.Name` 作为可选命名空间前缀来源；当 `config.App.Name` 去除首尾空白后非空时，token version 缓存 key MUST 为 `<app.name>:auth:user:token_version:{<user_id>}`，Refresh Token 会话记录 key MUST 为 `<app.name>:auth:session:{<user_id>}:<session_id>`，用户活跃会话索引 key MUST 为 `<app.name>:auth:user:sessions:{<user_id>}`；当 `config.App.Name` 去除首尾空白后为空时，系统 MUST 使用无前缀业务格式 `auth:user:token_version:{<user_id>}`、`auth:session:{<user_id>}:<session_id>` 和 `auth:user:sessions:{<user_id>}`。系统 MUST NOT 读取、写入或回退到旧格式 `<prefix>:auth:user:<user_id>:token_version`、`<prefix>:auth:session:<session_id>` 或 `<prefix>:auth:user:<user_id>:sessions`。`{<user_id>}` MUST 作为 Redis Cluster hash tag 保留在 key 中，使同一用户的会话 key、用户会话索引 key 和 token version key 能在脚本操作中落入同一 hash slot；服务名前缀位于 hash tag 外部，不得破坏同一用户多 key Lua 脚本的 cluster slot 一致性。用户活跃会话索引 MUST 使用 Redis ZSet，member MUST 使用 `session_id`，score MUST 使用该会话过期时间的 Unix 时间戳。系统 MUST 以 Redis session key 的实际 TTL 推导会话过期时间，并使会话 payload 中的 `ExpiresAt`、session key TTL 和用户活跃会话索引 score 保持一致。系统 MUST 在写入、读取或删除该用户活跃会话索引时，按当前 Unix 时间戳执行过期 member 清理。系统 MUST 为用户活跃会话索引设计过期或清理策略，避免没有活跃会话的 ZSet key 和已过期 `session_id` 长期残留。系统 MUST 使用 Redis Lua 脚本执行 Refresh Token 会话轮换，并使用 Redis Lua 脚本配合 `UNLINK` 执行用户全部会话删除。

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
- **Then** 系统 MUST 将 `aegiscore-user-services:auth:session:{<user_id>}:<session_id>` 保存为带 TTL 的会话记录
- **Then** 系统 MUST 将 `session_id` 写入 `aegiscore-user-services:auth:user:sessions:{<user_id>}` ZSet
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

#### Scenario: App name prefixes new authentication Redis keys
- **Given** `config.App.Name` 去除首尾空白后为 `aegiscore-user-services`
- **When** 系统保存 token version 缓存、Refresh Token 会话记录或用户活跃会话索引
- **Then** 系统 MUST 使用 `aegiscore-user-services:auth:user:token_version:{<user_id>}` 作为 token version 缓存 key
- **Then** 系统 MUST 使用 `aegiscore-user-services:auth:session:{<user_id>}:<session_id>` 作为 Refresh Token 会话记录 key
- **Then** 系统 MUST 使用 `aegiscore-user-services:auth:user:sessions:{<user_id>}` 作为用户活跃会话索引 key
- **Then** 这些 key MUST 仍通过 `{<user_id>}` hash tag 落入同一 Redis Cluster hash slot

#### Scenario: Empty app name keeps unprefixed new Redis keys
- **Given** `config.App.Name` 去除首尾空白后为空
- **When** 系统保存 token version 缓存、Refresh Token 会话记录或用户活跃会话索引
- **Then** 系统 MUST 使用 `auth:user:token_version:{<user_id>}` 作为 token version 缓存 key
- **Then** 系统 MUST 使用 `auth:session:{<user_id>}:<session_id>` 作为 Refresh Token 会话记录 key
- **Then** 系统 MUST 使用 `auth:user:sessions:{<user_id>}` 作为用户活跃会话索引 key
- **Then** 系统 MUST NOT 使用代码级默认服务名补齐 Redis key 前缀

#### Scenario: Legacy Redis keys are ignored
- **Given** Redis 中只存在旧格式 `auth:session:<session_id>`、`auth:user:<user_id>:sessions` 或 `auth:user:<user_id>:token_version`
- **When** 系统读取 Refresh Token 会话、用户会话索引或 token version 缓存
- **Then** 系统 MUST 只查询新格式 `<prefix>:auth:session:{<user_id>}:<session_id>`、`<prefix>:auth:user:sessions:{<user_id>}` 和 `<prefix>:auth:user:token_version:{<user_id>}`
- **Then** 系统 MUST NOT 回退读取旧格式 key
- **Then** 系统 MUST 按会话不存在或 token version cache miss 处理旧格式数据

#### Scenario: Logout current session removes indexed member and stale members
- **Given** 请求已通过 Access Token 认证
- **Given** 认证上下文包含当前 `user_id` 和 `session_id`
- **Given** `config.App.Name` 去除首尾空白后为 `aegiscore-user-services`
- **When** 调用方请求退出当前设备
- **Then** 系统 MUST 删除 `aegiscore-user-services:auth:session:{<user_id>}:<session_id>` 对应的 Redis 会话记录
- **Then** 系统 MUST 从 `aegiscore-user-services:auth:user:sessions:{<user_id>}` ZSet 移除当前 `session_id`
- **Then** 系统 MUST 清理该 ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member

#### Scenario: Rotate session uses Redis Lua atomically
- **Given** Refresh Token 轮转已启用
- **Given** Redis 中存在旧 Refresh Token 对应的 `<prefix>:auth:session:{<user_id>}:<old_session_id>` 会话记录
- **When** 系统执行 Refresh Token 会话轮换
- **Then** 系统 MUST 通过 Redis Lua 脚本原子校验旧会话归属用户、旧 `session_id` 和 `token_version`
- **Then** 同一 Lua 脚本 MUST 写入 `<prefix>:auth:session:{<user_id>}:<new_session_id>` 并设置 TTL
- **Then** 同一 Lua 脚本 MUST 更新 `<prefix>:auth:user:sessions:{<user_id>}` ZSet，写入新 `session_id`、移除旧 `session_id` 并清理过期 member
- **Then** 同一 Lua 脚本 MUST 删除旧会话 key
- **Then** 系统 MUST NOT 使用 WATCH/MULTI 重试循环作为该轮换的实现方式

#### Scenario: Rotate session rejects missing or mismatched old session
- **Given** Refresh Token 轮转已启用
- **Given** 旧会话 key 不存在，或旧会话 payload 的 `user_id`、`session_id`、`token_version` 与调用方校验结果不一致
- **When** 系统执行 Refresh Token 会话轮换 Lua 脚本
- **Then** 系统 MUST 拒绝轮换并返回 token 无效或等价认证失败响应
- **Then** 系统 MUST NOT 创建新会话 key
- **Then** 系统 MUST NOT 删除不匹配的旧会话 key

#### Scenario: Concurrent rotate session succeeds once
- **Given** Refresh Token 轮转已启用
- **Given** 多个请求并发使用同一个旧 Refresh Token 执行刷新
- **When** 这些请求同时调用 Redis Lua 轮换脚本
- **Then** 系统 MUST 最多只允许一个请求成功消费旧会话并创建新会话
- **Then** 其他请求 MUST 因旧会话不存在或状态不匹配而失败
- **Then** Redis 中 MUST NOT 同时保留多个由同一个旧会话成功轮换产生的新会话

#### Scenario: Logout all sessions unlinks non-expired indexed sessions
- **Given** 请求已通过 Access Token 认证
- **Given** `config.App.Name` 去除首尾空白后为 `aegiscore-user-services`
- **When** 调用方请求退出全部设备
- **Then** 系统 MUST 通过 Redis Lua 脚本先清理 `aegiscore-user-services:auth:user:sessions:{<user_id>}` ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member
- **Then** 同一 Lua 脚本 MUST 从该 ZSet 读取仍未过期的 `session_id`
- **Then** 同一 Lua 脚本 MUST 使用 `UNLINK` 删除读取到的 `aegiscore-user-services:auth:session:{<user_id>}:<session_id>` 会话记录
- **Then** 同一 Lua 脚本 MUST 使用 `UNLINK` 删除该用户活跃会话索引 key
- **Then** 系统 MUST NOT 使用可能阻塞 Redis 主线程的批量 `DEL` 删除用户全部会话 payload

#### Scenario: Stale index data does not inflate future session operations
- **Given** 用户活跃会话 ZSet 中存在已过期 `session_id` member
- **When** 系统创建新会话、删除当前会话、轮转会话或删除该用户全部会话
- **Then** 系统 MUST 在读取或写入业务相关 member 前清理按 score 已过期的 member
- **Then** 系统 MUST 避免长期遍历已过期残留作为活跃会话
- **Then** 后续会话统计、管理或审计能力 MUST 能基于清理后的索引语义区分活跃会话和过期残留

### Requirement: Refresh orchestration remains readable and strategy-oriented

用户会话控制能力 SHALL 保持 `AuthService.Refresh` 的高层用例编排清晰。刷新流程 MUST 将请求规范化、Refresh Token claims 解析、Refresh 会话校验、非轮转刷新、轮转刷新和轮转失败处理表达为职责明确的内部方法或组件边界。`AuthService.Refresh` MUST NOT 直接堆叠 Redis 旧会话校验、新会话创建、旧会话删除和补偿清理的低层细节。

#### Scenario: Refresh method selects a refresh strategy
- **Given** 调用方提交 Refresh Token 请求
- **When** `AuthService.Refresh` 处理请求
- **Then** 系统 MUST 先完成请求规范化、Refresh Token claims 解析和 Refresh 会话校验
- **Then** 系统 MUST 根据 Refresh Token 轮转配置选择非轮转刷新或轮转刷新策略
- **Then** 轮转策略的会话写入和撤销细节 MUST 位于职责明确的内部辅助方法、session lifecycle 组件或 repository 抽象内

#### Scenario: Non-rotation refresh keeps current session semantics
- **Given** Refresh Token 轮转未启用
- **Given** Refresh Token 校验通过且 Redis 中存在对应会话
- **When** 调用方请求刷新 token
- **Then** 系统 MUST 复用当前 `session_id` 签发新的 Access Token 和 Refresh Token
- **Then** 系统 MUST 保持现有响应信封、错误映射、JWT claims 和当前 Redis 会话存储契约不变
