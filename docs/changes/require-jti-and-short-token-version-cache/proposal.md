# Require JWT jti and shorten token version cache

## What

为所有新签发的 JWT 强制写入标准 `jti` claim，并在解析阶段拒绝缺少 `jti` 的旧 token。同时将用户服务默认 `token_version` Redis 缓存 TTL 从 `5m` 缩短为 `30s`。

本变更使用 `github.com/golang-jwt/jwt/v5` 的标准字段 `jwtv5.RegisteredClaims.ID` 承载 `jti`，不新增自定义 `JTI` 字段，避免重复表达 JWT 标准 claim。

签发影响范围包括：

- access token
- refresh token
- password_change token

解析影响范围包括：

- `ParseToken`
- `ParseRefreshToken`
- `ParsePasswordChangeToken`

## Why

当前 [common/security/auth/jwt.go](/Users/liubisen/Desktop/sander/Project/my/AegisCore/common/security/auth/jwt.go:39) 的 `Claims` 嵌入了 `jwtv5.RegisteredClaims`，具备承载标准 `jti` 的能力，但签发逻辑没有填充 `RegisteredClaims.ID`。这会导致每个 token 没有稳定唯一标识，后续无法在不撤销整批 `user_id + token_version` 的情况下设计单 token 撤销、审计定位或安全事件追踪。

当前 [user-service/configs/config.yaml](/Users/liubisen/Desktop/sander/Project/my/AegisCore/user-service/configs/config.yaml:44) 将 `token_version_cache_ttl` 设置为 `5m`。正常改密和退出全部设备路径会主动刷新或删除 Redis token version 投影，因此旧 token 并不会在正常路径必然继续使用 5 分钟；但当 Redis 投影刷新和删除都失败时，旧缓存会形成最长 5 分钟的残余风险窗口。将默认 TTL 缩短到 `30s` 可以降低降级场景下旧 token 被继续接受的时间。

## Scope

包括：

- 在 `common/security/auth` 中新增缺失 token ID 的错误，例如 `ErrMissingTokenID`。
- 在 JWT 签发路径为 `RegisteredClaims.ID` 设置 UUID v7 字符串。
- 在 JWT 通用解析路径校验 `claims.ID` 非空，缺失时拒绝 token。
- 保持 `jti` 使用标准 claim 名，不新增自定义 JSON 字段。
- 将用户服务默认配置 `auth.token_version_cache_ttl` 从 `5m` 修改为 `30s`。
- 更新依赖默认 TTL 的测试断言。
- 补充 JWT 测试，覆盖新签发 token 包含合法 UUID v7 `jti`，以及缺少 `jti` 的旧 token 被拒绝。

不包括：

- 不实现 Redis token denylist、blacklist 或单 token 撤销存储模型。
- 不新增强制撤销单个 token 的 HTTP API。
- 不修改 `/auth/logout` 与 `/auth/logout-all` 的路由语义。
- 不修改 access token、refresh token 或 password_change token 的 TTL。
- 不修改 Redis key schema、Ent schema、Atlas migration 或数据库表结构。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Compatibility

本变更明确不兼容旧 token。

部署后，所有缺少 `jti` 的存量 access token、refresh token 和 password_change token 都会在 JWT parse 阶段被拒绝。用户需要重新登录获取新 token。Redis 中已有的旧 refresh session 记录即使仍存在，也无法通过缺少 `jti` 的旧 refresh token 继续使用。

推荐将本变更作为安全升级发布，并在部署说明中明确“发布后需要重新登录”。如果运维窗口允许，可以在部署后清理认证 refresh session Redis key，减少旧会话垃圾数据残留。

## Acceptance Criteria

- 新签发的 access token、refresh token 和 password_change token 都包含非空 `jti`。
- 解析新签发 token 后，`claims.ID` 为合法 UUID v7 字符串。
- 缺少 `jti` 的旧 access token 被 `ParseToken` 拒绝。
- 缺少 `jti` 的旧 refresh token 被 `ParseRefreshToken` 拒绝。
- 缺少 `jti` 的旧 password_change token 被 `ParsePasswordChangeToken` 拒绝。
- JWT claim 中不出现自定义 `JTI` JSON 字段，只使用标准 `jti`。
- 用户服务默认 `auth.token_version_cache_ttl` 为 `30s`。
- 相关配置加载测试、HTTP 默认配置测试和 JWT 单元测试通过。
