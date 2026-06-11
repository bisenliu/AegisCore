## Why

认证会话撤销当前把 PostgreSQL `token_version` 变更、Redis token version cache 刷新和 Redis refresh session 删除串成同步成功路径，导致 Redis 写失败时用户看到失败，但 PostgreSQL 安全状态已经部分生效。强制改密流程还会先更新凭证递增一次 `token_version`，再执行全部会话撤销递增一次，造成版本语义不清且扩大跨存储不一致窗口。

## What Changes

- 将 PostgreSQL `token_version` 定义为认证撤销和改密后的唯一安全事实源。
- 将强制改密调整为“凭证更新 + 状态恢复 + token version 递增”的唯一 PostgreSQL 事务边界，改密后不再通过会话撤销流程再次递增 `token_version`。
- 将 Redis token version cache 刷新和 refresh session 删除调整为基于目标 `token_version` 的幂等投影/补偿动作。
- 当 Redis 投影失败时，系统应主动删除旧 token version cache 或允许认证中间件/会话组件回源 PostgreSQL，避免旧 access token 因旧缓存继续放行至 TTL。
- 调整退出全部设备和改密后的失败语义：PostgreSQL 版本变更成功后，Redis 投影失败不得把已经生效的安全状态报告为业务未完成；应记录日志并进入可重试补偿。
- 保持 HTTP 路由、请求/响应 JSON 结构、响应信封、配置字段、数据库 schema、migration 和 Redis key 格式兼容。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 修改 token version 撤销、强制改密和 Redis token version cache 回源/补偿要求，明确 PostgreSQL 为撤销事实源、Redis 为可重建投影。

## Impact

- 影响代码：`user-services/internal/features/auth/app/service.go`、`user-services/internal/features/auth/app/sessions.go`、`user-services/internal/features/auth/app/ports.go`、`user-services/internal/features/auth/infra/redis/session_store.go` 及相关单元测试。
- 可能影响代码：`user-services/internal/features/auth/app/credentials.go`、`user-services/internal/features/auth/infra/postgres/credential_store.go`，用于确保改密只在凭证更新边界递增一次 `token_version`。
- API 影响：HTTP 路由、请求体、响应结构和错误码枚举保持不变；Redis 投影失败后的业务结果语义会更正为“DB 安全状态成功后返回成功并补偿”，不再返回“失败但状态已变更”。
- 配置影响：无；继续使用 `auth.token_version_cache_ttl`、`cache_redis` 和 `user_db`。
- 数据模型影响：无；不修改 Ent schema、Atlas migration 或 Redis key 格式。
