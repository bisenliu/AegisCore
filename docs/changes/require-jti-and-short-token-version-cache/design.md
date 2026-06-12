# Design

## Overview

本变更分为两个独立但都属于认证安全收紧的小调整：

```text
common/security/auth
  -> 强制 JWT 标准 jti claim

user-service/configs/config.yaml
  -> 缩短 token_version Redis cache 默认 TTL
```

`jti` 使用 `jwtv5.RegisteredClaims.ID`，因为 `github.com/golang-jwt/jwt/v5` 已经将该字段映射为 JWT 标准 claim `jti`。`Claims` 不需要新增字段：

```go
type Claims struct {
    UserID       string `json:"user_id"`
    TokenVersion int64  `json:"token_version"`
    SessionID    string `json:"session_id"`
    jwtv5.RegisteredClaims
}
```

实现时只在签发和解析路径补齐约束。

## JWT Signing

`JWTService.sign` 负责 access、refresh 和 password_change token 的共同签发逻辑，因此应在这里统一生成 `jti`。

推荐实现：

```go
claims := Claims{
    UserID:       input.UserID,
    TokenVersion: input.TokenVersion,
    SessionID:    input.SessionID,
    RegisteredClaims: jwtv5.RegisteredClaims{
        ID:        tokenID.String(),
        Issuer:    s.issuer,
        Audience:  audienceClaim(s.audience),
        Subject:   subject,
        ExpiresAt: jwtv5.NewNumericDate(expiresAt),
    },
}
```

生成位置在所有输入校验之后、`jwtv5.NewWithClaims` 之前。`common/security/auth/jwt.go` 已经引入 `github.com/google/uuid` 用于校验 `user_id`，无需新增 UUID 依赖。

推荐在构造 claims 前先生成 UUID v7：

```go
tokenID, err := uuid.NewV7()
if err != nil {
    return "", fmt.Errorf("generate jwt jti: %w", err)
}
```

然后将 `tokenID.String()` 写入 `RegisteredClaims.ID`。使用 UUID v7 的原因是它保留 UUID 唯一性，同时具备时间有序特征，后续用于审计或撤销记录检索时更友好。

每次调用签发方法都生成新的 UUID，不能复用 session ID、user ID、token version 或其他业务字段作为 `jti`。原因：

- `jti` 表示 token 实例 ID，而不是会话 ID。
- 同一 session 内可能多次刷新 token，每个 token 需要独立标识。
- 后续如果实现 token denylist，可以按 token 实例精确撤销。

## JWT Parsing

新增错误：

```go
// ErrMissingTokenID 表示 JWT 缺少标准 jti claim，无法唯一标识 token。
ErrMissingTokenID = errors.New("jwt jti is required")
```

校验放在 `JWTService.parse`，使三类 token 共用同一强制约束：

```go
if claims.ID == "" {
    return nil, ErrMissingTokenID
}
```

推荐校验顺序：

1. 验证 secret。
2. 解析并验证签名、过期、issuer、audience。
3. 校验 token 有效。
4. 校验 `claims.ID` 非空。
5. 校验 `claims.UserID` 非空且是 UUID。
6. 返回 claims，由 `ParseToken`、`ParseRefreshToken`、`ParsePasswordChangeToken` 继续校验 subject、token_version、session_id。

是否校验 `claims.ID` 是合法 UUID：

- 建议校验为合法 UUID，因为本服务签发时使用 `uuid.NewV7()`，解析阶段只需要拒绝缺失或畸形 `jti`，不需要把 UUID version 作为兼容性约束。
- 如果实现该校验，应新增错误，例如 `ErrInvalidTokenID = errors.New("jwt jti is invalid")`。
- 如果希望变更更小，可以第一版只校验非空；测试仍应验证新签发 token 的 `claims.ID` 是合法 UUID。

本 change 推荐实现合法 UUID 校验；签发侧继续生成 UUID v7。

## Backward Compatibility

本变更不兼容旧 token。

缺少 `jti` 的 token 在 `parse` 阶段被拒绝，影响：

- 旧 access token 无法访问受保护 API。
- 旧 refresh token 无法换取新 token。
- 旧 password_change token 无法完成强制改密。

这符合本次安全收紧目标。上线前应确认产品和运维接受“发布后需要重新登录”的行为。

为了避免隐藏兼容窗口，不增加“如果缺少 `jti` 则自动补齐”或“仅对 access token 强制”的逻辑。

## Token Version Cache TTL

将用户服务默认配置从：

```yaml
token_version_cache_ttl: 5m
```

调整为：

```yaml
token_version_cache_ttl: 30s
```

该配置仍保持正数时长。当前配置校验 [common/runtime/config/validation.go](/Users/liubisen/Desktop/sander/Project/my/AegisCore/common/runtime/config/validation.go:150) 使用 `validatePositiveDuration`，因此本变更不支持通过 `0` 禁用缓存。

缩短 TTL 后的行为：

- 正常路径：改密和退出全部设备仍主动刷新 Redis token version 投影，旧 access token 应立即被拒绝。
- Redis 投影刷新失败但删除成功：后续请求回源 PostgreSQL，旧 access token 被拒绝。
- Redis 投影刷新失败且删除失败：旧缓存最多保留 `30s`，比原先 `5m` 风险窗口更短。

## Tests

JWT 测试建议：

- `TestJWTServiceSignTokens` 断言 access token 解析后 `claims.ID` 非空、`uuid.Parse(claims.ID)` 成功且 UUID version 为 7。
- 同一测试或新增子测试断言 refresh token 和 password_change token 的 `claims.ID` 同样非空且是 UUID v7。
- 新增缺少 `jti` 的旧 access token 测试，期望 `ErrMissingTokenID`。
- 新增缺少 `jti` 的旧 refresh token 测试，期望 `ErrMissingTokenID`。
- 新增缺少 `jti` 的旧 password_change token 测试，期望 `ErrMissingTokenID`。
- 如实现 `ErrInvalidTokenID`，补充畸形 `jti` 测试。

配置测试建议：

- 更新 `common/runtime/config/loader_test.go` 中显式配置样例的期望值。
- 更新 `user-service/internal/bootstrap/http_test.go` 中默认配置 TTL 断言。
- 检查测试 YAML fixture 或字符串中是否仍写死 `token_version_cache_ttl: 5m`，按测试意图决定是否改为 `30s`。

## Documentation

本变更不需要更新 `docs/ARCHITECTURE.md` 的结构规则，因为没有新增目录、模块、feature 边界或运行时资源。

如实现后希望记录安全行为变化，可以在 `docs/DEVELOPMENT.md` 或认证 API 文档中补充：

- JWT 必须包含标准 `jti`。
- 发布后旧 token 不兼容。
- 默认 token version cache TTL 为 `30s`。

## Rollout

建议发布流程：

1. 在 release note 中说明本次认证安全升级会让旧 token 失效。
2. 部署应用变更。
3. 观察认证失败日志，确认缺少 `jti` 的旧 token 被拒绝符合预期。
4. 可选清理 Redis 中旧 refresh session key，减少存量会话垃圾数据。
5. 关注数据库 token version 回源压力，确认 TTL 缩短到 `30s` 后 Redis miss 增长仍在可接受范围。
