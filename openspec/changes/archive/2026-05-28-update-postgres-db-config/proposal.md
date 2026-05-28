## Why

当前 PostgreSQL 配置以 `database.postgres.<db>.dsn` 的方式为每个数据库重复声明完整 DSN，主机、端口、账号和密码等公共连接信息会在 `user_db`、`pay_db`、`common_db` 中重复维护。变更需要支持用户给出的扁平配置格式，通过共享连接参数和多个数据库名生成各数据库连接，降低配置重复和出错概率。

## What Changes

- 修改 `shared-infrastructure` 能力的 PostgreSQL 配置要求，支持以下结构：`host`、`port`、`username`、`password`、`user_db_name`、`pay_db_name`、`common_db_name`。
- 更新配置结构体、配置加载默认值/校验逻辑和 PostgreSQL 连接池创建代码，使系统基于上述字段构造 `user_db`、`common_db` 和 `pay_db` 的连接配置。
- 更新示例 YAML 配置，改为使用新的 PostgreSQL 配置格式。
- 保持现有 Fx 命名连接池对 `user_db` 和 `common_db` 的可用性；`pay_db` 仅作为配置可解析的数据库名，不在本变更中引入支付业务能力或支付仓储逻辑。
- **BREAKING**：旧的 `database.postgres.<name>.dsn` map 配置格式不再作为当前目标格式。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-infrastructure`：PostgreSQL 配置要求从按数据库 DSN map 调整为共享连接参数加数据库名称字段，并要求配置加载和基础设施初始化据此创建所需连接池。

## Impact

- 影响代码：`common/config/`、`common/infrastructure/postgres.go`、`user-services/configs/config.yaml`，以及相关配置/基础设施测试。
- 影响配置：`database.postgres` 的 YAML 结构将从按数据库名称嵌套 DSN 改为扁平连接参数与数据库名字段。
- 影响运行时：PostgreSQL 连接仍通过 `pgx` 打开，并继续在 Fx 生命周期启动时 ping、停止时关闭。
- 不影响 HTTP API 响应契约、用户资料查询 API 路由或 Ent schema；不需要修改 `user-services/ent/` 生成代码。
