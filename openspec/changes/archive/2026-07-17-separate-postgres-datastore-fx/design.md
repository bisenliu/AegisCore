## Context

当前 `common/runtime/datastore/postgres_fx.go` 同时实现 PostgreSQL 打开、批量构造、启动探测、失败回收、关闭和 Fx provider。`NewPostgres` 接收整个 `PostgresConfigs` 与 `fx.Lifecycle`，并通过 `NewPostgresPools` 返回的 map 获取单个 pool。`user-service/internal/providers/postgres.go` 即使只需要 `primary_db`，仍依赖该批量 primitive。

`common/runtime/datastore/redis_fx.go` 已采用能力包内共置方式。本次 PostgreSQL Fx adapter 延续相同结构，并通过 `NewPostgres` 与框架无关的 `OpenPostgres` 区分职责。

## Goals / Non-Goals

**Goals:**

- 核心 datastore 提供一次只构造一个 PostgreSQL 连接池的普通 Go constructor。
- 保留资源名称、配置默认值、单资源 PING timeout、失败回滚、关闭错误和日志语义。
- Fx adapter 只注册一个资源的 Ping/Close，不遍历配置 map。
- user-service 显式从配置 map 选择 `primary_db`，并通过 `NewPrimaryDB` 注册该资源。
- user-service 显式从配置 map 选择 `cache_redis`，并通过 `NewCacheRedis` 注册该资源。
- Fx adapter 与核心实现共置，并通过文件和 symbol 边界区分职责。

**Non-Goals:**

- 不保留旧 API、兼容 wrapper、deprecated alias 或批量 constructor。
- 不新增或自动创建 `pay_db` 等尚无真实消费者的资源。
- 不改变 Redis 核心构造行为、配置契约或观测语义。
- 不改变数据库 schema、migration、部署或 HTTP 契约。

## Decisions

### Decision: 直接返回单资源连接池

`datastore.OpenPostgres(name, cfg)` 直接返回 `*sql.DB`。`PingPostgres` 和 `ClosePostgres` 显式接收资源名称与连接池，由核心层统一保证具名错误、默认探测 timeout 和 Ping 失败关闭语义，避免仅为包装连接池或测试注入而增加具体 owner、取值方法和 opener 抽象。

### Decision: constructor 不执行启动探测

constructor 负责打开 pool 和应用默认值；`PingPostgres` 负责启动探测。普通 Go 调用方可显式控制启动时机，Fx adapter 则在 `OnStart` 调用同一契约。Ping 失败时立即调用 `ClosePostgres`，并通过 `errors.Join` 保留探测与关闭错误。

### Decision: Fx adapter 在 datastore 包内共置

PostgreSQL 和 Redis Fx adapter 与核心实现共同放在 `common/runtime/datastore`。PostgreSQL 使用 `NewPostgres` 注册 lifecycle，框架无关 constructor 使用 `OpenPostgres`，避免职责和签名冲突，并与仓库内 config、logger 等 runtime capability 的 Fx 文件布局保持一致。

### Decision: 服务 composition 显式选择配置

user-service 的 `NewPrimaryDB` 和 `NewCacheRedis` 分别从配置 map 选择 `primary_db` 与 `cache_redis` 的单份配置，再调用通用 Fx constructor。配置中存在其他 entry 不会触发构造；未来只有出现真实消费者时才新增对应 constructor。

## Risks / Trade-offs

- [Risk] 删除旧导出 API 会使所有调用点编译失败。Mitigation：同一变更内迁移仓库内全部直接调用并通过全仓搜索确认无残留。
- [Risk] Ping 失败后 Fx 可能继续执行 Stop。Mitigation：保持 `database/sql.DB.Close` 可重复调用的底层语义，测试验证失败回滚只发生在启动错误路径。
- [Risk] 迁移 Redis adapter 扩大文件移动范围。Mitigation：不改变其 API 行为，只更新包路径和测试归属。

## Migration Plan

1. 创建并验证 OpenSpec artifacts。
2. 用单资源 constructor 和具名 Ping/Close 函数替换 PostgreSQL 批量 API，并迁移核心测试。
3. 将 PostgreSQL 与 Redis Fx adapter 作为独立 `*_fx.go` 文件共置在 datastore 包并迁移 adapter 测试。
4. 将 user-service 改为显式 `NewPrimaryDB` composition 并更新测试。
5. 运行相关测试、架构 lint、依赖检查、lint 和 verify。
6. 验证完成后归档 change，将 delta 合并到主规格。

回滚必须整体恢复旧 API、adapter 路径和服务 composition，不提供运行时双轨兼容。
