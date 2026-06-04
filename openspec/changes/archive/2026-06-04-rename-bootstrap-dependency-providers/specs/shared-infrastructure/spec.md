## ADDED Requirements

### Requirement: Name bootstrap runtime dependency providers by Fx responsibility

用户服务 bootstrap 中返回 `fx.Out` 并声明运行时依赖集合的 provider SHALL 使用 `Provide*` 命名。PostgreSQL pools provider MUST 命名为 `ProvidePostgresPools`，Redis clients provider MUST 命名为 `ProvideRedisClients`，Ent clients provider MUST 命名为 `ProvideEntClients`。该命名调整 MUST 只表达 Fx provider 职责，不得改变配置路径、Fx named injection、连接创建数量、启动 ping、停止 close 或 Ent client lifecycle 行为。

#### Scenario: User service module declares named datastore providers clearly
- **Given** 用户服务 Fx module 需要声明 PostgreSQL pools、Redis clients 和 Ent clients
- **When** 维护者查看 `user-services/internal/bootstrap/app.go` 的 provider 列表
- **Then** PostgreSQL pools provider MUST 使用 `ProvidePostgresPools`
- **Then** Redis clients provider MUST 使用 `ProvideRedisClients`
- **Then** Ent clients provider MUST 使用 `ProvideEntClients`
- **Then** 普通 controller、service、repository 和 HTTP 构造函数 MUST NOT 因被 `fx.Provide` 注册而统一改为 `Provide*`

#### Scenario: Preserve datastore runtime contracts after provider rename
- **Given** 配置中存在 `redis.cache_redis`、`postgres.user_db`、`postgres.common_db`、`redis.queue_redis` 和 `postgres.pay_db`
- **When** 用户服务启动并解析 bootstrap runtime dependency providers
- **Then** 系统 MUST 继续只创建 `cache_redis` Redis client
- **Then** 系统 MUST 继续只创建 `user_db` 和 `common_db` PostgreSQL 连接池
- **Then** 系统 MUST NOT 创建 `queue_redis` Redis client 或 `pay_db` PostgreSQL 连接池
- **Then** Redis 和 PostgreSQL provider MUST 继续注册启动 ping 与停止 close lifecycle

#### Scenario: Preserve Ent client wiring after provider rename
- **Given** Fx 容器中存在具名 `user_db` 和 `common_db` PostgreSQL 连接池
- **When** Ent clients provider 被 Fx 构造
- **Then** 系统 MUST 继续创建具名 `user_db` 和 `common_db` Ent clients
- **Then** repository MUST 继续接收具名 `user_db` Ent client
- **Then** Ent clients 停止 lifecycle MUST 保持不变
- **Then** 实现 MUST NOT 在 repository、service 或 controller 层直接打开数据库连接
