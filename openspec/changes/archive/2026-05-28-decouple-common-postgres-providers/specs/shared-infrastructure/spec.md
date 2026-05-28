## MODIFIED Requirements

### Requirement: Provide shared runtime dependencies through Fx

系统必须通过 `common/infrastructure.Module` 提供配置、日志和 Redis client。PostgreSQL 共享基础能力必须支持按调用方指定的单个数据库配置创建具名连接池，并注册启动 ping 与停止 close 生命周期；具体服务必须在自己的 Fx module 中声明需要哪些具名 PostgreSQL 连接池。PostgreSQL 连接池必须基于共享连接参数和目标数据库名称构造连接，而不是读取每个数据库的独立 DSN。

#### Scenario: Provide common dependencies
- **Given** Fx app 包含 `common/infrastructure.Module`
- **When** Fx 解析依赖
- **Then** module 提供 `*config.Config`、`*slog.Logger` 和 `*redis.Client`
- **Then** module 不得固定提供所有已知业务数据库的具名 PostgreSQL 连接池

#### Scenario: Register one PostgreSQL lifecycle
- **Given** 调用方为一个目标数据库提供逻辑注入名和 PostgreSQL 数据库配置
- **When** PostgreSQL 单库 provider 被调用
- **Then** 系统创建一个具名 `*sql.DB` 连接池
- **Then** 该连接池必须连接到调用方指定的目标数据库
- **Then** Fx app 启动时 ping 该数据库
- **Then** Fx app 停止时关闭该数据库连接池

#### Scenario: User service declares required PostgreSQL pools
- **Given** 用户服务运行时需要 `user_db` 和 `common_db`
- **When** Fx app 包含用户服务模块
- **Then** 用户服务模块必须声明并提供具名 `user_db` 和 `common_db` PostgreSQL 连接池
- **Then** `user_db` 连接池必须连接到 `user_db_name` 指定的数据库
- **Then** `common_db` 连接池必须连接到 `common_db_name` 指定的数据库

#### Scenario: Service does not connect unused PostgreSQL pools
- **Given** 某个服务只声明需要 `common_db`
- **When** Fx app 启动该服务
- **Then** 系统必须只创建该服务声明的 PostgreSQL 连接池
- **Then** 系统不得因为其他配置字段存在而连接未声明的业务数据库

#### Scenario: Pay database name is configurable without adding payment runtime dependency
- **Given** 配置中存在 `pay_db_name`
- **When** `common/infrastructure.Module` 提供当前用户服务运行时依赖
- **Then** 系统必须允许配置对象读取 `pay_db_name`
- **Then** 系统不得仅因为存在 `pay_db_name` 而创建支付数据库连接池、支付业务 API、支付 repository 或支付 Ent client
