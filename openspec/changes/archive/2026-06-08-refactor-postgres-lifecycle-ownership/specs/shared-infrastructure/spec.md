## ADDED Requirements

### Requirement: Own PostgreSQL pool lifecycle once

共享基础设施 SHALL 对每个具名 PostgreSQL `*sql.DB` 连接池建立单一 lifecycle owner。该 owner MUST 负责创建连接池、使用对应 `postgres.<name>` 配置执行启动 ping、在 Fx stop 时关闭连接池，并在多连接池创建失败时回滚已创建但无法进入完整运行时的连接池。基于这些连接池创建的 Ent clients MUST NOT 重复拥有底层共享 `*sql.DB` 连接池的 close 职责。停止或回滚错误 MUST 保留失败的具名 PostgreSQL 实例或 Ent client 上下文。

#### Scenario: PostgreSQL helper owns raw pool lifecycle
- **Given** 调用方声明需要逻辑名为 `user_db` 的 PostgreSQL 连接池
- **When** 共享 PostgreSQL helper 创建 `postgres.user_db` 对应的 `*sql.DB`
- **Then** 系统 MUST 注册该连接池的启动 ping lifecycle
- **Then** 系统 MUST 注册该连接池的停止 close lifecycle
- **Then** 该连接池 MUST NOT 依赖 Ent client lifecycle 才能释放底层 `*sql.DB` pool

#### Scenario: Ent clients do not duplicate raw pool close ownership
- **Given** Fx 容器中存在具名 `user_db` 和 `common_db` PostgreSQL 连接池
- **Given** 用户服务 bootstrap 基于这些连接池创建具名 Ent clients
- **When** Fx app 停止并执行 datastore 与 Ent lifecycle
- **Then** Ent client cleanup MUST NOT 对同一个底层共享 `*sql.DB` pool 建立第二个 close owner
- **Then** 每个底层 PostgreSQL pool MUST 只通过约定的 PostgreSQL pool owner 释放

#### Scenario: Multiple PostgreSQL pool creation rolls back consistently
- **Given** 用户服务运行时声明 `user_db` 和 `common_db` PostgreSQL 连接池
- **Given** `user_db` 连接池已创建
- **When** `common_db` 连接池创建失败
- **Then** 系统 MUST 关闭已经创建但无法进入完整运行时的 `user_db` 连接池
- **Then** 返回错误 MUST 保留 `common_db` 创建失败的具名上下文
- **Then** 系统 MUST NOT 留下会在后续 stop 流程中重复关闭已回滚 `user_db` pool 的模糊 lifecycle 责任

#### Scenario: Named PostgreSQL runtime behavior is preserved
- **Given** 配置中存在 `postgres.user_db`、`postgres.common_db` 和 `postgres.pay_db`
- **When** 用户服务启动并声明 `user_db` 与 `common_db` PostgreSQL 连接池
- **Then** 系统 MUST 继续只创建 `user_db` 和 `common_db` 连接池
- **Then** 系统 MUST NOT 创建未声明的 `pay_db` 连接池
- **Then** `user_db` 和 `common_db` 的连接参数、连接池参数和 ping timeout MUST 继续来自对应 `postgres.<name>` 配置

#### Scenario: Close errors preserve named context
- **Given** 用户服务运行时已经创建具名 `user_db` 和 `common_db` PostgreSQL 连接池
- **When** Fx app stop 关闭这些连接池且任一 close 操作失败
- **Then** stop 错误 MUST 包含失败 PostgreSQL pool 的具名上下文
- **Then** 多个 close 失败同时发生时，stop 错误 MUST 保留每个失败 pool 的底层错误
