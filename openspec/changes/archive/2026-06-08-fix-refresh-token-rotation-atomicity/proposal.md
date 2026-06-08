## Why

启用 RefreshTokenRotation 时，当前刷新流程会先删除旧 Redis session，再签发新 token 并创建新 session；一旦新 token 签发或新 session 写入失败，旧 Refresh Token 已经失效而新 Refresh Token 又无法返回，用户会被动掉线。

该问题影响 `user-session-control` 中 Refresh Token 轮换的可靠性和一致性，需要将轮换语义从“先撤销旧会话再创建新会话”修正为“确保新凭据可用后再撤销旧会话”，或通过 Redis 原子操作消除中间失败窗口。

## What Changes

- 调整启用 RefreshTokenRotation 时的刷新流程，避免在新 token 或新 session 不可用前永久撤销旧 Refresh Token 会话。
- 明确 Refresh Token 轮换必须保证成功响应中的新 Refresh Token 与 Redis 新 session 一致可用。
- 明确失败路径不得使已通过校验的旧 Refresh Token 无故失效，除非该失败发生在安全目标要求的原子防重放提交中且系统能返回确定失败语义。
- 评估并实现更稳妥的轮换方式：优先先签发新 token、写入新 session，成功后再撤销旧 session；如需要严格防重放，则使用 Redis 事务或 Lua 脚本将旧 session 校验、创建新 session、删除旧 session 和索引更新组合为原子轮换。
- 增加覆盖 token 签发失败、Redis 新 session 写入失败和旧 session 删除失败的测试，验证不会出现旧 token 已失效且新 token 未返回的被动掉线场景。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 修正 Refresh Token 轮换要求，消除先删除旧 session 再创建新 session 导致的非原子性失败窗口。

## Impact

- 主要影响代码：`user-services/internal/service/auth_service.go` 中 `Refresh` 与 `issueTokenPair` 的调用顺序和错误处理。
- 可能影响代码：`user-services/internal/service/session_store.go` 及 `repository.AuthSessionRepository`/Redis 实现，如选择 Redis 事务或 Lua 脚本承载原子轮换。
- API 路由、请求体、响应信封、JWT claims、Redis key 格式和数据库 schema 不应发生兼容性变更。
- 外部可观察行为变化：启用 RefreshTokenRotation 时，刷新失败不应因为内部 token 签发或 Redis 写入失败而使用户当前有效 Refresh Token 被提前撤销。
