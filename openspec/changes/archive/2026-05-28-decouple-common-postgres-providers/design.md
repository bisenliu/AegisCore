## Context

AegisCore 当前通过 `common/infrastructure.Module` 提供配置、日志、Redis 和 PostgreSQL 连接池。PostgreSQL 部分由 `common/infrastructure/postgres.go` 中的 `NewPostgresPools` 一次性创建 `user_db` 与 `common_db` 两个具名 `*sql.DB`，并在 Fx 生命周期中启动 ping、停止 close。

这个设计在只有用户服务时可以工作，但共享层已经开始承载业务服务数据库集合：新增服务如果只需要 `common_db` 或自己的数据库，也会被迫复用会打开 `user_db` 的 provider，或继续把服务专属数据库名称加入 `common`。变更需要保持基础设施能力共享，同时把“某个服务需要哪些数据库”的决定移回服务模块。

## Goals / Non-Goals

**Goals:**

- `common` 提供可复用的单个 PostgreSQL 连接池创建和 Fx 生命周期注册能力。
- 服务模块显式声明需要的具名数据库连接池，避免共享层固定初始化所有已知数据库。
- 用户服务继续能注入 `user_db` 和 `common_db`，保持现有 Ent client 装配和 repository 行为。
- 保持连接池参数、启动 ping、停止 close、日志字段和错误语义可测试。

**Non-Goals:**

- 不新增业务服务、支付能力、认证能力或新的 HTTP API。
- 不修改 Ent schema，不手写 `user-services/ent/` 生成代码。
- 不改变 API 响应信封、错误码或 controller/service/repository 分层。
- 不在本次变更中设计完整的跨服务数据库配置注册中心。

## Decisions

1. 将 `common` 的 PostgreSQL provider 从批量输出改为单库构建原语。

   `common/infrastructure/postgres.go` 保留打开数据库、设置连接池参数、注册 Fx 生命周期的公共实现，并提供面向调用方的单库创建函数。调用方传入逻辑注入名和目标数据库配置，函数只返回或注册一个 `*sql.DB`。

   选择该方案而不是继续扩展 `PostgresPools`，因为 `PostgresPools` 的字段会随着服务增长而膨胀，并让 `common` 了解业务数据库拓扑。

2. 由用户服务模块提供 `user_db` 与 `common_db` 的 Fx 输出。

   用户服务可以在 `user-services/internal/bootstrap` 或相邻 package 中定义自己的 `fx.Out`，内部调用 `common` 的单库构建能力来输出具名 `*sql.DB`。这样 `entclient.NewClients` 仍通过 `name:"user_db"` 和 `name:"common_db"` 注入，不需要改变 repository 层。

   选择服务侧 provider 而不是在 `common/infrastructure.Module` 传入数据库清单，是为了让每个服务的依赖图在服务模块内可见，并避免共享模块变成参数化的全局数据库装配器。

3. 保留共享基础设施模块中的非业务共享依赖。

   `common/infrastructure.Module` 继续提供 `NewConfig`、`NewLogger` 和 `NewRedisClient`。PostgreSQL 的通用 helper 仍位于 `common`，但服务需要在自己的 Fx module 中选择要调用哪些数据库 provider。

   选择不把 PostgreSQL helper 从 `common` 移到用户服务，是因为连接池配置、ping/close 生命周期和日志仍是跨服务共享基础能力。

4. 保持当前用户服务兼容。

   用户服务运行时仍应在启动时连接 `user_db` 和 `common_db`，并在停止时关闭两个连接池。外部配置字段、Ent client 注入名、HTTP API 和响应契约不变。

## Risks / Trade-offs

- [Risk] Fx provider 从 `common` 移到服务模块后，服务遗漏数据库 provider 会导致依赖解析失败。→ Mitigation: 在用户服务模块中集中注册 PostgreSQL providers，并增加 bootstrap/provider 级测试覆盖具名 `*sql.DB` 输出。
- [Risk] 当前配置对象仍包含具体数据库名称字段，可能继续限制未来服务扩展。→ Mitigation: 本次先解耦 provider 初始化边界；后续如需要新增服务数据库，再单独提出配置别名或 map 化变更。
- [Risk] 拆分 provider 后生命周期注册顺序变化可能影响启动失败清理。→ Mitigation: 单库 helper 必须在打开后立即注册生命周期，启动 ping 失败由 Fx 生命周期返回错误，停止时逐个关闭已注册连接池。

## Migration Plan

1. 在 `common/infrastructure/postgres.go` 中引入单库 PostgreSQL 创建/注册 API，并删除或停止从 `common/infrastructure.Module` 提供固定 `NewPostgresPools`。
2. 在用户服务模块新增服务侧 PostgreSQL provider，输出具名 `user_db` 与 `common_db` 连接池。
3. 保持 `entclient.NewClients` 的输入注入名不变，验证用户服务依赖图仍能解析。
4. 运行 `common` 和 `user-services` 测试。若需要回滚，可恢复 `common/infrastructure.Module` 对旧批量 provider 的注册，并移除服务侧 provider。

## Open Questions

- 暂无。未来服务如果需要动态数据库别名或 map 化配置，应通过单独 OpenSpec change 设计。
