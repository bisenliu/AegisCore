## MODIFIED Requirements

### Requirement: Separate token version cache from database lookup strategy

用户会话控制能力 SHALL 将 Redis token version cache 操作与 PostgreSQL token version 读取策略分离。Redis auth session repository MUST 只负责 Redis 会话记录、用户活跃会话索引、token version cache key 的读取、写入和删除；它 MUST NOT 直接依赖用户 repository 或在缓存未命中时自行回源 PostgreSQL。认证会话 service 组件或专门的 token version resolver MUST 组合 Redis cache 与 `UserTokenVersionStore`，并保持缓存未命中时回源 PostgreSQL、成功后尽力回填 Redis 缓存的行为。Redis 读取异常或缓存回填失败 MUST NOT 让系统依赖可能过期的缓存值放行 token；系统 MUST 回源 PostgreSQL 或返回安全失败。

#### Scenario: Cache hit uses Redis value without database lookup
- **Given** Redis 中存在某用户有效的 token version 缓存
- **When** 系统需要校验该用户的 token version
- **Then** 系统 MUST 使用 Redis 缓存中的版本值
- **Then** 系统 MUST NOT 查询 PostgreSQL 获取该用户的 token version

#### Scenario: Cache miss is resolved outside Redis repository
- **Given** Redis 中不存在某用户的 token version 缓存
- **When** 系统需要校验该用户的 token version
- **Then** Redis auth session repository MUST 只报告缓存未命中或等价结果
- **Then** 认证会话 service 组件或 token version resolver MUST 使用外部 `user_id` 回源 PostgreSQL 获取真实 `token_version`
- **Then** 回源成功后系统 MUST 尽力将真实版本回填 Redis 缓存

#### Scenario: Redis repository does not depend on user repository
- **Given** 用户服务运行时通过 Fx 构造 Redis auth session repository
- **When** 查看 Redis auth session repository 的构造参数和字段
- **Then** Redis auth session repository MUST 依赖具名 `cache_redis` Redis client、Redis key builder 和认证缓存 TTL 配置
- **Then** Redis auth session repository MUST NOT 持有 `UserRepository` 或 `UserTokenVersionStore`

#### Scenario: Invalid token version cache falls back through resolver
- **Given** Redis 中存在无法解析或非正数的 token version 缓存值
- **When** 系统需要校验该用户的 token version
- **Then** Redis auth session repository MUST 将该值视为无效缓存
- **Then** 认证会话 service 组件或 token version resolver MUST 回源 PostgreSQL 获取真实 `token_version`
- **Then** 回源成功后系统 MUST 尽力使用真实版本覆盖 Redis 缓存

#### Scenario: Redis cache read error falls back to database
- **Given** Redis token version cache 读取发生非缓存未命中错误
- **When** 系统需要校验 Access Token、Refresh Token 或受限改密 token 的 token version
- **Then** 系统 MUST 回源 PostgreSQL 获取真实 `token_version`
- **Then** 系统 MUST 使用 PostgreSQL 返回的真实版本完成当前 token version 判定
- **Then** 系统 MUST NOT 因 Redis 中可能存在旧缓存而跳过 PostgreSQL 判定

#### Scenario: Token version cache backfill failure does not fail validation after database read
- **Given** Redis token version cache 未命中或读取失败
- **Given** PostgreSQL 成功返回用户真实 `token_version`
- **When** 系统无法将该真实版本回填 Redis
- **Then** 系统 MUST 使用 PostgreSQL 版本完成当前 token version 判定
- **Then** 系统 MUST 记录可观测日志或补偿信号
- **Then** 系统 MUST NOT 因缓存回填失败把当前请求映射为内部错误

#### Scenario: External behavior remains compatible
- **Given** 调用方执行登录、刷新、修改密码、退出当前设备或退出全部设备流程
- **When** token version cache 与 PostgreSQL 读取策略完成解耦
- **Then** 系统 MUST 保持现有 HTTP 路由、请求体、响应信封、错误码和认证语义不变
- **Then** 系统 MUST 继续使用 `auth.token_version_cache_ttl` 控制 token version 缓存 TTL
- **Then** 系统 MUST NOT 修改 Ent schema、Atlas migration 或 Redis key 格式

### Requirement: Update user credentials through a credential update contract

系统 SHALL 在修改密码成功后通过用户仓储的凭证更新契约持久化新凭证。凭证更新 MUST 面向未软删除用户，MUST 在同一个 PostgreSQL 持久化边界内写入新的 `password_hash`、更新目标用户状态、递增 PostgreSQL 中的 `token_version`，并 MUST 返回更新后的 `token_version`。系统 MUST NOT 在该流程中读取或写入旧 `password` 字段。修改密码流程完成凭证更新后，系统 MUST 使用凭证更新返回的新 `token_version` 执行 Redis token version cache 刷新和 refresh session 删除投影，MUST NOT 再通过全部会话撤销流程二次递增 PostgreSQL `token_version`。

#### Scenario: Password change updates credentials and invalidates tokens
- **Given** 用户存在且 `deleted_at` 为 `NULL`
- **Given** 用户当前状态为 `300`
- **Given** 调用方持有有效的受限改密凭据
- **When** 调用方提交有效的新密码并完成密码哈希
- **Then** 系统 MUST 通过凭证更新契约写入新的 `password_hash`
- **Then** 系统 MUST 将用户状态更新为 `100`
- **Then** 系统 MUST 递增 PostgreSQL 中的 `token_version`
- **Then** 系统 MUST 使用更新后的 `token_version` 使旧凭据和旧认证会话不再通过版本校验

#### Scenario: Password change increments token version once
- **Given** 修改密码流程已验证受限改密凭据
- **When** 系统成功更新用户凭证并撤销旧认证会话
- **Then** PostgreSQL 中该用户的 `token_version` MUST 只递增一次
- **Then** Redis token version cache 刷新 MUST 使用凭证更新返回的 `token_version`
- **Then** 修改密码流程 MUST NOT 再调用会递增 PostgreSQL `token_version` 的全部会话撤销动作

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

#### Scenario: Password change remains successful when Redis projection fails after persistence
- **Given** 用户凭证更新已成功递增 PostgreSQL `token_version`
- **When** 系统无法将 Redis token version cache 写入为递增后的新版本或无法删除 Redis refresh session
- **Then** 系统 MUST 将 PostgreSQL 中的新 `token_version` 视为该次安全更新的事实源
- **Then** 系统 MUST 尝试删除旧 token version cache 或记录可重试补偿信号
- **Then** 系统 MUST NOT 将已经完成的修改密码报告为业务失败

### Requirement: Authentication sessions use repository abstraction with Redis implementation boundary

用户会话控制能力 SHALL 通过认证 app 层声明的 `authapp.AuthSessionStore` 抽象管理 token version cache、Refresh Token 会话和用户活跃会话索引，具体 Redis 实现 MUST 位于 `user-services/internal/features/auth/infra/redis` 包。service 层 MUST NOT 定义或持有 Redis session store 具体实现。Redis token version cache 写入 MUST 保持单调语义，较旧的投影写入 MUST NOT 覆盖较新的已缓存版本。缓存刷新、缓存删除和 refresh session 批量删除 MUST 能作为幂等补偿重复执行。

#### Scenario: Auth service depends on auth session repository abstraction
- **Given** 登录、刷新、退出当前设备、退出全部设备或修改密码流程需要访问会话状态
- **When** auth service 调用会话数据访问层
- **Then** auth service MUST 依赖 `authapp.AuthSessionStore` 或更高层 session lifecycle 组件
- **Then** auth service MUST 使用 `authdomain.AuthSession` 表达会话数据
- **Then** auth service MUST NOT 依赖 Redis client 或 `features/auth/infra/redis` 私有实现类型

#### Scenario: Session not found error remains mappable
- **Given** Redis 中不存在指定 Refresh Token 会话记录
- **When** auth service 读取会话
- **Then** Redis 实现 MUST 返回 `authdomain.ErrAuthSessionNotFound`
- **Then** auth service MUST 继续将该错误映射为未认证或 token 无效响应

#### Scenario: Token version mismatch remains mappable
- **Given** token claims 或会话记录中的 `token_version` 与服务端当前版本不一致
- **When** 系统校验 token version
- **Then** auth session store 或 lifecycle 组件 MUST 返回认证领域 token version mismatch 错误
- **Then** 系统 MUST 继续拒绝刷新、受保护请求或改密凭据校验

#### Scenario: Redis session storage behavior remains compatible
- **Given** `features/auth/infra/redis` 承载认证会话 Redis 实现
- **When** 系统创建、读取、删除或批量删除认证会话
- **Then** Redis key 格式、Refresh Token 会话 TTL、用户活跃会话 ZSet 和过期 member 清理行为 MUST 与迁移前保持一致
- **Then** token version 缓存未命中时 Redis 实现 MUST 只报告缓存未命中或等价结果，由认证会话 service 组件或 token version resolver 回源 PostgreSQL

#### Scenario: Token version cache write is monotonic
- **Given** Redis 中已缓存某用户较新的 token version
- **When** 旧请求或补偿任务尝试写入较旧或相等语义之外的过期 token version
- **Then** Redis 实现 MUST NOT 用较旧版本覆盖较新版本
- **Then** 后续 token version 校验 MUST 继续看到不小于 PostgreSQL 已生效撤销版本的缓存值，或回源 PostgreSQL

#### Scenario: Token version cache eviction supports compensation
- **Given** PostgreSQL 中用户 `token_version` 已递增
- **When** Redis token version cache 无法刷新为新版本
- **Then** 系统 MUST 尝试删除旧 token version cache 或记录等价补偿动作
- **Then** 后续认证校验在缓存未命中时 MUST 回源 PostgreSQL
- **Then** 旧 Access Token MUST NOT 因旧缓存继续通过版本校验直到原缓存 TTL 结束

## ADDED Requirements

### Requirement: Revoke sessions with PostgreSQL as source of truth

用户会话控制能力 SHALL 将 PostgreSQL 用户 `token_version` 作为用户级认证撤销的唯一安全事实源。退出全部设备可以通过认证会话组件递增 PostgreSQL `token_version` 产生新撤销版本；强制改密必须复用凭证更新返回的新撤销版本。Redis token version cache 刷新和 Redis refresh session 删除 SHALL 作为该版本的幂等投影执行，投影失败 MUST 可通过缓存驱逐、回源 PostgreSQL 或后台补偿恢复，不得要求跨 PostgreSQL 与 Redis 的同步原子提交。

#### Scenario: Logout all commits revocation before Redis projection
- **Given** 请求已通过 Access Token 认证
- **When** 用户执行退出全部设备
- **Then** 系统 MUST 先在 PostgreSQL 中递增该用户 `token_version` 并获得新版本
- **Then** 系统 MUST 使用该新版本刷新 Redis token version cache 并删除该用户全部 Redis refresh session
- **Then** Redis 投影失败 MUST NOT 回滚 PostgreSQL `token_version`

#### Scenario: Password change uses credential update as the only version boundary
- **Given** 修改密码流程已验证受限改密凭据
- **When** 用户凭证更新成功并返回新的 `token_version`
- **Then** 系统 MUST 使用该 `token_version` 撤销旧 token 和 refresh session
- **Then** 系统 MUST NOT 为同一次修改密码再次递增 PostgreSQL `token_version`

#### Scenario: Redis projection failure after revocation is compensated
- **Given** PostgreSQL 中用户 `token_version` 已经递增到新版本
- **When** Redis token version cache 刷新或 Redis refresh session 删除失败
- **Then** 系统 MUST 记录带 `user_id` 和 `token_version` 的错误日志或补偿信号
- **Then** 系统 MUST 尝试删除旧 token version cache 或依赖后续 token version 校验回源 PostgreSQL
- **Then** 系统 MUST 允许相同 `user_id` 和 `token_version` 的 Redis 投影动作被安全重试

#### Scenario: Middleware rejects stale access token after cache eviction
- **Given** 用户级撤销已在 PostgreSQL 中递增 `token_version`
- **Given** Redis 中旧 token version cache 已被删除或失效
- **When** 旧 Access Token 请求受保护 API
- **Then** 认证中间件的 token version validator MUST 回源 PostgreSQL 获取当前版本
- **Then** 系统 MUST 拒绝 token claims 中携带旧 `token_version` 的 Access Token

#### Scenario: No distributed transaction requirement
- **Given** 用户级撤销涉及 PostgreSQL 和 Redis
- **When** 系统实现退出全部设备或修改密码后的会话撤销
- **Then** 系统 MUST NOT 依赖跨 PostgreSQL 与 Redis 的分布式事务作为正确性前提
- **Then** 系统 MUST 通过 PostgreSQL 事实源、Redis 幂等投影和 token version 校验回源保证安全语义
