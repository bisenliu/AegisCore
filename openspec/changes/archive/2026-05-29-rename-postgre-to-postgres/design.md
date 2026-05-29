## Context

AegisCore 的共享基础设施由 `common/config` 负责加载 YAML 和 `AEGISCORE_` 环境变量，并由 `common/infrastructure` 基于具名配置创建 Redis 与 PostgreSQL 运行时依赖。当前 PostgreSQL 配置根节点和 `Config` 字段使用 `postgre` / `Postgre`，但 provider、DSN scheme、测试名称和通用术语均使用 `postgres` 或 `Postgres`，命名不一致。

该变更影响共享配置契约和用户服务示例配置，不涉及 HTTP API、controller/service/repository 分层、Ent schema 或数据库结构。

## Goals / Non-Goals

**Goals:**

- 将 YAML PostgreSQL 配置根节点统一为 `postgres`。
- 将环境变量覆盖路径统一为 `AEGISCORE_POSTGRES_<INSTANCE>_<FIELD>`。
- 将 Go 配置结构字段从 `Postgre` 更名为 `PostgresConfigs`，并保持现有 `Config.Postgres(name)` lookup API 语义。
- 更新用户服务配置样例和测试，使 `user_db`、`common_db`、`pay_db` 都从 `postgres.<name>` 读取。
- 更新迁移工具相关约定，明确从项目配置派生目标连接信息时使用 `postgres.user_db`。

**Non-Goals:**

- 不保留 `postgre` 兼容读取路径；这是一次配置契约更名。
- 不修改 HTTP API、响应信封、错误码或路由。
- 不修改 Ent schema、生成代码或 SQL migration 文件。
- 不新增支付数据库连接池、支付 API 或健康检查聚合能力。

## Decisions

- 使用 `postgres` 作为唯一配置根节点，而不是同时接受 `postgre` 和 `postgres`。
  - 理由：避免长期存在两个等价配置入口导致运维和测试混淆。
  - 备选：保留兼容 alias。该方案降低短期迁移成本，但会扩大配置加载逻辑并增加后续清理成本。
- 保留 `Config.Postgres(name)` 方法名，将底层 map 字段更名为 `PostgresConfigs` 并使用 `mapstructure:"postgres"`。
  - 理由：方法名已经是正确领域术语，调用方不需要改变抽象语义；Go 不允许字段和方法同名，因此内部字段不能命名为 `Postgres`。
  - 备选：新增 `PostgresConfig(name)` 方法。该方案不必要，会增加 API 表面积。
- PostgreSQL provider 和用户服务 bootstrap 继续通过逻辑实例名 `user_db`、`common_db` 声明依赖。
  - 理由：本次只更改配置根节点，不改变服务声明哪些数据库，也不改变 Fx 注入名。
  - 备选：重命名实例名。该方案会扩大变更范围，并影响与业务数据库语义相关的测试和文档。
- 迁移工具约定只更新命名路径，不引入运行时依赖。
  - 理由：Atlas 迁移仍应通过 `DATABASE_URL` 或从配置组装连接 URL，不能启动 Fx、Redis、HTTP server 或 Ent runtime client。

## Risks / Trade-offs

- 现有部署仍使用 `postgre:` 会导致 PostgreSQL 命名实例读取为空，服务启动时找不到目标数据库配置。
  - 缓解：在 proposal、spec 和 tasks 中明确标记 breaking change，并更新样例配置和测试覆盖。
- 环境变量名从 `AEGISCORE_POSTGRE_...` 变为 `AEGISCORE_POSTGRES_...`，旧环境变量不再生效。
  - 缓解：测试覆盖 `AEGISCORE_POSTGRES_USER_DB_*` 覆盖路径，并在实现说明中同步更新文档。
- 文档或测试中残留 `postgre` 会造成使用者误判真实配置契约。
  - 缓解：实现时全仓搜索 `postgre` / `Postgre`，仅保留历史 change artifact 或明确说明旧名的文本。
