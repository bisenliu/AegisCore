## Context

当前仓库由 `common` 和 `user-services` 两个 Go module 组成，`shared-infrastructure` 能力集中在 `common/config/` 与 `common/infrastructure/`。现有 `Config.Database.Postgres` 是 `map[string]PostgresConfig`，每个数据库项包含 `driver`、`dsn`、连接池大小和 ping timeout。`common/infrastructure.NewPostgresPools` 固定打开 `user_db` 与 `common_db` 两个连接池，并通过 Fx name 提供给用户服务 Ent client。

用户希望改为参考如下 PostgreSQL 配置结构：`host`、`port`、`username`、`password`、`user_db_name`、`pay_db_name`、`common_db_name`。该结构把公共连接参数集中声明，把不同数据库的差异缩小为数据库名字段。

## Goals / Non-Goals

**Goals:**

- 将 `database.postgres` 配置模型从按数据库 DSN map 调整为共享连接参数加数据库名字段。
- 在配置加载阶段校验 `host`、`port`、`username`、`user_db_name`、`common_db_name` 等必需字段，并为连接池参数保留现有默认值。
- 在 PostgreSQL 基础设施初始化时基于新配置构造 `user_db` 和 `common_db` DSN，并保持现有 Fx 命名输出不变。
- 让 `pay_db_name` 可在配置中声明并被校验，为后续支付能力预留数据库名，但不创建支付业务逻辑。
- 更新示例配置和测试，覆盖新格式、缺失必填字段和连接串构造。

**Non-Goals:**

- 不新增支付服务、支付 repository、支付 Ent client 或 `pay_db` Fx 连接池消费方。
- 不修改 HTTP API 路由、controller/service/repository 分层或响应信封。
- 不修改 Ent schema，不手写 `user-services/ent/` 生成代码。
- 不保留旧 `database.postgres.<name>.dsn` 配置格式的兼容解析，除非实现阶段发现已有测试或运行约束必须保留。

## Decisions

1. 使用单个 `PostgresConfig` 表示 `database.postgres`。

   当前 map 格式适合每个数据库完全独立配置，但会重复公共连接参数。新结构明确要求公共 host/port/username/password 与多个数据库名字段，因此将 `DatabaseConfig.Postgres` 改为结构体可以让配置模型与目标 YAML 一致，并简化校验逻辑。

   备选方案是保留 map 并增加一个 common section，但这会偏离用户给出的示例格式，并继续要求调用方理解内部数据库 key。

2. 在 `common/config` 中生成命名数据库连接配置，而不是让基础设施直接拼接散落字段。

   `common/infrastructure/postgres.go` 仍应通过类似 `cfg.Postgres("user_db")` 的方式取得已规范化的数据库连接信息。实现可通过新增方法或内部 helper 从扁平 PostgreSQL 配置生成 `user_db`、`common_db` 的 `driver`、DSN 和连接池参数，避免基础设施层直接了解 YAML 结构细节。

   备选方案是在 `openPostgres` 中直接读取 `cfg.Database.Postgres.UserDBName` 并拼 DSN，但会把配置解释逻辑扩散到基础设施层。

3. 保持 Fx 输出只包含当前实际使用的 `user_db` 和 `common_db`。

   能力地图中明确支付服务或 `pay_db` 相关业务仍是候选未来能力。虽然新配置包含 `pay_db_name`，本变更只保证该字段可配置和校验，不新增 `pay_db` 连接池输出，以避免提前引入未使用依赖导致启动需要额外数据库可用。

   备选方案是立即打开 `pay_db` 连接池，但这会让当前用户服务启动依赖新增支付数据库，属于不必要的运行时耦合。

4. 使用 `pgx` 驱动和 URL DSN 构造。

   现有代码使用 `github.com/jackc/pgx/v5/stdlib` 和 `sql.Open(dbCfg.Driver, dbCfg.DSN)`。新实现继续默认 `driver=pgx`，通过 host/port/username/password/database name 生成 `postgres://...?...` DSN，以最小化基础设施变化。

## Risks / Trade-offs

- [Risk] 配置格式是破坏性变更，旧 YAML 会加载失败。→ Mitigation：在 proposal、spec 和 tasks 中明确 BREAKING，并更新示例配置与配置加载测试。
- [Risk] 密码或用户名包含特殊字符时手写 DSN 可能转义错误。→ Mitigation：使用 `net/url` 或等价标准库方式构造 DSN，避免字符串拼接。
- [Risk] 环境变量覆盖扁平字段时 key 变化，部署环境需要同步更新。→ Mitigation：文档和配置示例使用 `database.postgres.<field>` 结构，保持 `AEGISCORE_` 前缀映射规则不变。
- [Risk] `pay_db_name` 被校验但不建立连接池可能让读者误解为支付能力已可用。→ Mitigation：在配置文档和任务中说明它仅作为数据库名配置，不新增支付业务能力。
