## ADDED Requirements

### Requirement: Configure datastore ping timeouts consistently

系统 MUST 允许 Redis 和 PostgreSQL 命名实例分别声明 ping timeout，并在 Fx lifecycle 启动检查中使用对应实例的配置值。

#### Scenario: Redis uses configured ping timeout
- **Given** `redis.cache_redis` 配置声明 ping timeout
- **When** Redis 单实例 provider 注册启动 lifecycle
- **Then** 系统必须使用 `redis.cache_redis` 的 ping timeout 创建 ping context
- **Then** 系统不得使用隐藏的固定 ping timeout 覆盖实例配置

#### Scenario: PostgreSQL keeps configured ping timeout
- **Given** `postgres.user_db` 配置声明 ping timeout
- **When** PostgreSQL 单实例 provider 注册启动 lifecycle
- **Then** 系统必须使用 `postgres.user_db` 的 ping timeout 创建 ping context

### Requirement: Provide explicit named datastore Fx helpers

系统 MUST 提供 opt-in 的命名 Redis 和 PostgreSQL Fx provider helper，以减少服务侧重复 wiring。helper 必须只为调用方声明的单个命名实例创建 client 或连接池，不得自动连接配置中存在但未声明的实例。

#### Scenario: Provide one named Redis helper
- **Given** 调用方声明需要逻辑名为 `cache_redis` 的 Redis client
- **When** 调用方使用命名 Redis helper 组装 Fx app
- **Then** 系统必须只创建 `redis.cache_redis` 对应的 `*redis.Client`
- **Then** 系统必须注册该 client 的启动 ping 和停止 close lifecycle

#### Scenario: Provide one named PostgreSQL helper
- **Given** 调用方声明需要逻辑名为 `user_db` 的 PostgreSQL 连接池
- **When** 调用方使用命名 PostgreSQL helper 组装 Fx app
- **Then** 系统必须只创建 `postgres.user_db` 对应的 `*sql.DB`
- **Then** 系统必须注册该连接池的启动 ping 和停止 close lifecycle

#### Scenario: Do not connect undeclared datastores through helpers
- **Given** 配置中存在 `redis.queue_redis` 和 `postgres.pay_db`
- **When** 服务只声明 `cache_redis` Redis helper 和 `user_db` PostgreSQL helper
- **Then** 系统不得创建 `queue_redis` Redis client
- **Then** 系统不得创建 `pay_db` PostgreSQL 连接池

### Requirement: Keep shared logger access concurrency-safe

系统 MUST 保证共享 logger 的默认实例读写在并发场景下是安全的，并继续支持通过 context 输出 `trace-id` 字段。

#### Scenario: Set and read default logger concurrently
- **Given** 测试或启动流程并发调用默认 logger 设置和 context logger 获取 API
- **When** 这些调用同时发生
- **Then** 系统不得产生数据竞争
- **Then** 获取到的 logger 必须可用于正常写日志

#### Scenario: Logger package remains modular
- **Given** 维护者需要修改 logger factory、context helper 或 file writer
- **When** 查看 `common/logger` 包
- **Then** 不同职责的实现必须组织在聚焦文件中
- **Then** 文件组织不得要求修改一个聚合大文件才能维护无关职责

### Requirement: Provide configurable shared HTTP middleware policies

系统 MUST 允许服务通过 options 配置共享 CORS 与 trace-id 中间件策略，并保留现有便捷 middleware 的可用性。

#### Scenario: Configure CORS policy
- **Given** 服务声明允许的 origins、methods、headers、exposed headers、credentials 和 max age
- **When** 服务使用可配置 CORS middleware
- **Then** 响应必须按声明策略写入 CORS headers
- **Then** 使用 origin 反射策略时响应必须设置 `Vary: Origin`

#### Scenario: Preserve trace id header contract
- **Given** HTTP 请求包含合法 `X-Trace-ID` header
- **When** trace-id middleware 处理请求
- **Then** 系统必须将该值写入 Gin context、Go context 和响应 `X-Trace-ID` header
- **Then** 日志字段必须继续使用 `trace-id`

#### Scenario: Reject unsafe inbound trace id values
- **Given** HTTP 请求包含超长或不符合策略的 `X-Trace-ID` header
- **When** trace-id middleware 使用配置的校验策略处理请求
- **Then** 系统必须生成替代 trace id 或按配置拒绝该值
- **Then** 系统不得把不安全的原始值写入日志字段或响应 header
