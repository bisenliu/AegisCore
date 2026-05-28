## Why

`common/infrastructure.NewPostgresPools` 当前固定创建 `user_db` 和 `common_db`，使共享基础设施层知道并强绑定业务服务数据库集合。随着新增服务出现，复用该 provider 会迫使服务连接不需要的数据库，并让 `common` 逐渐反向依赖业务结构。

## What Changes

- 将 `common` 的 PostgreSQL 能力调整为“按名称创建并注册单个连接池”的公共基础能力。
- 移除共享层固定初始化所有已知数据库的职责，避免在 `common/infrastructure.Module` 中提供全局数据库大礼包。
- 由具体服务模块声明自己需要哪些具名 PostgreSQL 连接池，例如用户服务继续声明 `user_db` 和 `common_db`。
- 保持现有配置加载、连接池参数、启动 ping、停止 close 的运行时语义。
- 不新增业务 API、数据模型或 Ent schema。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: PostgreSQL provider 的规格从固定提供 `user_db` 与 `common_db`，调整为 `common` 提供单库连接创建/生命周期注册能力，服务模块按需组合具名数据库连接池。

## Impact

- 影响 `common/infrastructure/postgres.go` 和 `common/infrastructure/module.go` 的 provider 边界。
- 影响 `user-services/internal/bootstrap` 或相邻基础设施装配代码，由用户服务显式注册所需数据库连接池。
- 影响 `openspec/specs/shared-infrastructure/spec.md` 中关于 PostgreSQL Fx provider 的要求。
- HTTP API、响应信封、错误码、Ent 数据模型和外部配置字段保持兼容；用户服务仍需要 `user_db` 和 `common_db` 时应继续按既有名称注入。
