# Tasks

## Preparation

- [x] 阅读 `AGENTS.md` 和 `docs/ARCHITECTURE.md`，确认本仓库不新增 `openspec/` 或 `docs/opsx/`。
- [x] 确认当前 token version 校验路径为 auth middleware 每请求调用 validator，validator 先查 Redis，miss 后回源 PostgreSQL。
- [x] 确认当前默认 `auth.token_version_cache_ttl` 是 `30s`，本变更不调整 Redis TTL。
- [x] 确认 `DeleteAllUserSessions` 当前不使用 Redis `SCAN`，也不在退出全部设备主路径直接执行 `ZREMRANGEBYSCORE`。
- [x] 确认本次实现范围只覆盖 token version 本地 L1 cache 和同实例 singleflight 合并。

## Local Cache

- [x] 在 `common/runtime/localcache` 中新增通用短 TTL 本地缓存实现。
- [x] 为 `New`、`Get`、`Set`、`Delete` 补充中文公开注释，包含用途、参数说明、返回值说明和使用示例。
- [x] 在 auth token version validator 中使用 `common/runtime/localcache`。
- [x] 定义短 TTL 默认值，例如 `defaultTokenVersionLocalCacheTTL = time.Second`。
- [x] 使用 `sync.Map` 存储 `userID -> version/expiresAt`。
- [x] 实现 `Get`、`Set`、`Delete` 方法。
- [x] 非正 TTL 回退默认值，避免永久本地缓存。
- [x] 不新增后台 goroutine、workerpool、scheduler 或外部缓存库。

## Validator

- [x] 将 `tokenVersionValidator` 扩展为持有本地 cache 和 `singleflight.Group`。
- [x] 保持 `NewValidator` 的入参和返回类型兼容现有 Fx provider。
- [x] 在 `ValidateTokenVersion` 中优先读取本地 cache。
- [x] 本地 miss 时使用 `singleflight.Group.Do(userID, ...)` 合并并发加载。
- [x] `singleflight` closure 内二次检查本地 cache。
- [x] miss loader 复用现有 `Current(ctx, users, sessions, userID)`，保持 Redis/PostgreSQL fallback 语义不变。
- [x] loader 成功后写入本地 cache。
- [x] loader 返回错误时不缓存错误。
- [x] 本地命中后仍使用 `commonauth.ValidateTokenVersion` 执行 mismatch 判断。

## Invalidation

- [x] 定义 auth application 内部窄接口，例如 `TokenVersionLocalInvalidator`。
- [x] 让 `tokenVersionValidator` 实现 `InvalidateTokenVersion(userID string)`。
- [x] `InvalidateTokenVersion` 删除本地 cache，并调用 `singleflight.Group.Forget(userID)`。
- [x] 调整 Fx wiring，让 session lifecycle 可拿到同一个 validator 实例的 invalidator 能力。
- [x] 在强制改密、退出全部设备或 `RevokeUserSessionsAtVersion` 相关路径中，刷新 token version projection 前后失效本实例本地 cache。
- [x] 保持跨实例失效广播不在本变更范围内，并确保默认 TTL 控制多副本 stale window。

## Tests

- [x] 新增本地 cache 单元测试，覆盖 miss、hit、expire、delete 和非正 TTL fallback。
- [x] 新增或更新 token version validator 测试，覆盖首次 miss 加载、二次本地命中、TTL 过期重新加载。
- [x] 覆盖 token version mismatch 仍返回 `ErrTokenVersionMismatch`。
- [x] 覆盖 loader 错误不写入本地 cache。
- [x] 覆盖同一 user ID 并发校验只触发一次 loader。
- [x] 覆盖不同 user ID 并发校验不会互相合并。
- [x] 覆盖 `InvalidateTokenVersion` 后下一次校验会重新加载。
- [x] 如现有 middleware 测试未覆盖，补充 expired JWT 不调用 validator 的断言。

## Guardrails

- [x] 不新增 `openspec/` 或 `docs/opsx/` 工件。
- [x] 不修改 JWT claim schema、token parse/sign、token TTL 或 Redis token version TTL 默认值。
- [x] 不修改 Redis key schema、Redis Lua 脚本、Ent schema、Atlas migration 或 PostgreSQL 表结构。
- [x] 不新增 Redis denylist、blacklist、eventbus、outbox、MQ 或跨实例失效广播。
- [x] 不修改 `DeleteAllUserSessions`、`CreateSession`、`RotateSession` 或 `DeleteSession` 的 Redis 清理算法。
- [x] 不新增横向 `internal/shared`、`internal/service`、`internal/repository` 或兜底 helper 包。
- [x] `common/runtime/localcache` 不包含 token version、user ID、auth、Redis key 或其他业务语义。
- [x] 保持 application 层不导入 Gin、Redis client、Ent、SQL、workerpool 或 scheduler。
- [x] 保持源码注释为中文，日志消息为英文。

## Verification

- [x] 运行 auth application validator 相关测试：

```bash
cd user-service
go test ./internal/features/auth/application/validators
```

- [x] 运行 common localcache 测试：

```bash
cd common
go test ./runtime/localcache
```

- [x] 运行 auth application sessions/command 相关测试：

```bash
cd user-service
go test ./internal/features/auth/application/sessions ./internal/features/auth/application/command
```

- [x] 运行 auth middleware 测试：

```bash
cd common
go test ./http/middleware
```

- [x] 运行 auth Redis adapter 测试，确认未破坏 Redis session 和 token version projection 语义：

```bash
cd user-service
go test ./internal/features/auth/infrastructure/redis
```

- [x] 运行架构边界检查：

```bash
make architecture-lint
```

- [x] 检查没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 检查 application 层没有引入禁止依赖：

```bash
rg -n "redis/go-redis|gin-gonic|entgo.io|database/sql|runtime/workerpool|runtime/scheduler" user-service/internal/features/auth/application
```

## Release Notes

- [x] 说明 token version 校验新增同实例短 TTL 本地缓存，默认 TTL 约 `1s`。
- [x] 说明已过期 JWT 仍在 parse 阶段被拒绝，本地 cache 不延长 token 有效期。
- [x] 说明改密和退出全部设备在本实例内会主动失效本地 cache；多副本最坏 stale window 为本地 TTL。
- [x] 说明本变更没有修改 Redis session 清理算法，Redis 大 key 清理风险需依赖 slowlog、ZCARD 和 purge task 耗时另行观测。
