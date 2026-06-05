## Why

当前用户服务认证会话 Redis key 没有按服务身份隔离，后续多个服务或多环境复用 Redis 时容易发生 key 命名冲突。服务启动日志也缺少服务名、运行环境和时区等关键运行时上下文，不利于排查部署配置是否按预期生效。

## What Changes

- 认证会话相关 Redis key MUST 使用 `config.App.Name` 作为前缀。
- 不新增 `app.name` 校验，不设置代码级默认服务名；当 `config.App.Name` 为空时，Redis key 前缀按空字符串处理。
- `NewHTTPServer` 启动日志在保留现有字段和日志行为的基础上，追加服务名、运行环境和系统时区字段。
- 不抽取 `/api/v1`、`/auth`、`/users` 等路由路径常量，不修改 Swagger 注释路径。
- 不改变 JWT issuer、HTTP API 路由、响应信封、Redis 连接实例、PostgreSQL/Ent wiring 或数据库 schema。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 认证会话 Redis key 命名改为以 `config.App.Name` 为前缀，且不校验、不默认填充该前缀。
- `http-service-runtime`: HTTP server 启动日志追加服务名、运行环境和时区上下文，同时保留现有启动日志字段。

## Impact

- 影响代码：`user-services/internal/repository/redis` 中认证会话 Redis key 构造逻辑，`user-services/internal/bootstrap/server.go` 中 HTTP server 启动日志。
- 影响配置：复用现有 `app.name`、`app.environment` 和 `system.timezone` 字段，不新增配置项。
- 外部行为：HTTP API、Swagger 路径、JWT claims、数据库 schema 不变。
- 数据兼容性：Redis key 前缀变化会影响既有 Redis 会话和 token version 缓存读取；变更部署后，旧无前缀 key 不再被新逻辑读取，用户可能需要重新登录或等待旧 key TTL 过期。
