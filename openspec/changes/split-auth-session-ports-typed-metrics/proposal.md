## Why

auth application 当前把 Redis token version 投影、refresh session 生命周期和批量会话撤销集中在同一个 `AuthSessionStore` port 中，调用方会被迫依赖超过自身需要的存储面。全部会话撤销还把 PostgreSQL token version 递增与 Redis 投影刷新、refresh session 删除串联在应用层，但接口没有清晰表达强一致或最终一致边界，容易让安全失效语义和补偿责任变得模糊。

## What Changes

- 拆分 auth application 的会话相关 port，使 token version cache、refresh session store 和 token version 持久化仓储分别表达最小依赖面。
- 明确全部会话撤销的应用层契约：用户 token version 递增是安全失效的主事实，Redis token version 投影刷新和 refresh session 物理删除必须有清晰错误处理、日志和可补偿语义。
- 保持现有 HTTP API、响应结构、Redis key schema、数据库 schema 和 Prometheus 指标名称/label key 不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`：认证会话能力需要显式约束 auth application port 的职责边界和全部会话撤销的一致性语义。

## Impact

- 影响 Go 代码：`user-service/internal/features/auth/application/ports.go`、`application/sessions/`、`application/validators/`、`application/command/`、Redis session store adapter、Fx provider 和相关测试。
- 影响安全边界：全部会话撤销继续通过 token version 递增使旧 access token 失效，并通过 refresh session 撤销阻止旧 refresh token 续签；实现必须明确投影失败时的返回、日志和补偿策略。
- 不影响 auth metrics 调用契约、HTTP API、OpenAPI 文档、数据库 schema、Ent migration、Redis key schema、Casbin policy、部署清单或 Grafana/Prometheus 资产。
