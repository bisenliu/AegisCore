## ADDED Requirements

### Requirement: Separate token version cache from database lookup strategy

用户会话控制能力 SHALL 将 Redis token version cache 操作与 PostgreSQL token version 读取策略分离。Redis auth session repository MUST 只负责 Redis 会话记录、用户活跃会话索引、token version cache key 的读取、写入和删除；它 MUST NOT 直接依赖用户 repository 或在缓存未命中时自行回源 PostgreSQL。认证会话 service 组件或专门的 token version resolver MUST 组合 Redis cache 与 `UserTokenVersionRepository`，并保持缓存未命中时回源 PostgreSQL、成功后回填 Redis 缓存的行为。

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
- **Then** 回源成功后系统 MUST 将真实版本回填 Redis 缓存

#### Scenario: Redis repository does not depend on user repository
- **Given** 用户服务运行时通过 Fx 构造 Redis auth session repository
- **When** 查看 Redis auth session repository 的构造参数和字段
- **Then** Redis auth session repository MUST 依赖具名 `cache_redis` Redis client、Redis key builder 和认证缓存 TTL 配置
- **Then** Redis auth session repository MUST NOT 持有 `UserRepository` 或 `UserTokenVersionRepository`

#### Scenario: Invalid token version cache falls back through resolver
- **Given** Redis 中存在无法解析或非正数的 token version 缓存值
- **When** 系统需要校验该用户的 token version
- **Then** Redis auth session repository MUST 将该值视为无效缓存
- **Then** 认证会话 service 组件或 token version resolver MUST 回源 PostgreSQL 获取真实 `token_version`
- **Then** 回源成功后系统 MUST 使用真实版本覆盖 Redis 缓存

#### Scenario: External behavior remains compatible
- **Given** 调用方执行登录、刷新、修改密码、退出当前设备或退出全部设备流程
- **When** token version cache 与 PostgreSQL 读取策略完成解耦
- **Then** 系统 MUST 保持现有 HTTP 路由、请求体、响应信封、错误码和认证语义不变
- **Then** 系统 MUST 继续使用 `auth.token_version_cache_ttl` 控制 token version 缓存 TTL
- **Then** 系统 MUST NOT 修改 Ent schema、Atlas migration 或 Redis key 格式
