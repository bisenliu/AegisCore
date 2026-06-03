## ADDED Requirements

### Requirement: Keep user service resource wiring explicit after bootstrap split
系统 SHALL 在 `user-services/internal/bootstrap` 中继续显式声明用户服务运行时资源装配，包括具名 Redis client、具名 PostgreSQL pool 和 Ent runtime client。资源装配可移动到聚焦文件，但 MUST 保持 `cache_redis`、`user_db` 和 `common_db` 的配置路径、Fx named injection 和 lifecycle 行为不变。

#### Scenario: Datastore wiring remains explicit
- **Given** 用户服务 Fx module 初始化运行时依赖
- **When** bootstrap 文件按职责拆分
- **Then** 用户服务 MUST 继续只声明 `redis.cache_redis`、`postgres.user_db` 和 `postgres.common_db`
- **Then** 系统 MUST NOT 因文件拆分自动连接 `redis.queue_redis`、`postgres.pay_db` 或其他未声明实例
- **Then** Redis 和 PostgreSQL provider MUST 继续注册启动 ping 与停止 close lifecycle

#### Scenario: Ent clients stay at service bootstrap boundary
- **Given** 用户 repository 和认证会话 repository 依赖 Ent client
- **When** Ent client wiring 移动到聚焦文件
- **Then** `user_db` 和 `common_db` Ent clients MUST 继续在用户服务 bootstrap 边界创建
- **Then** repository MUST 继续接收具名 `user_db` Ent client
- **Then** 实现 MUST NOT 为拆分 bootstrap 而在 repository、service 或 controller 层直接打开数据库连接
