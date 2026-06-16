# Design

## Overview

本变更只优化 auth token version 校验热点路径：

```text
JWT parse
  -> token version validator
  -> local L1 cache
  -> Redis token_version projection
  -> PostgreSQL token_version fallback
```

JWT 过期校验仍在 auth middleware 的 `jwtService.ParseToken` 阶段完成。只有 parse 成功后才进入 token version validator，因此本地缓存不会让已过期 token 继续有效。

本地缓存能力由 `common/runtime/localcache` 提供，属于无业务语义的 runtime primitive；auth feature application 层只使用该 primitive 保存 `userID -> token_version` 的短期值。Auth application 仍只依赖标准库、消费侧 ports、common runtime primitive 和 common security auth 原语，不导入 Redis client、Gin、Ent 或 infrastructure adapter。Redis token version 投影仍由 auth Redis adapter 拥有。

`common/runtime/localcache` 不包含 token version、user ID、auth、Redis key 或任何业务语义。它只提供泛型短 TTL 本地缓存能力。

## Component Placement

当前 token version 校验位于：

```text
user-service/internal/features/auth/application/validators/token_version_validator.go
```

本变更将通用缓存 primitive 放入：

```text
common/runtime/localcache/cache.go
```

Auth validator 继续位于同一 package 内，避免扩大 auth feature 目录调整范围：

```text
user-service/internal/features/auth/application/validators/token_version_cache.go
```

推荐结构：

```go
type tokenVersionValidator struct {
    users    authapplication.UserTokenVersionStore
    sessions authapplication.AuthSessionStore
    cache    *localcache.Cache[string, int64]
    group    singleflight.Group
}
```

`NewValidator` 继续返回 `commonauth.TokenVersionValidator`，不改变服务级 provider 或 common middleware 的依赖契约。

## Local Cache Model

通用 cache key 是任意 comparable 类型，value 是任意类型。Auth 使用 `string -> int64`：

```go
cache := localcache.New[string, int64](time.Second)
```

`common/runtime/localcache` 第一版使用 `sync.Map` 加短 TTL，避免引入新缓存库：

```go
type Cache[K comparable, V any] struct {
    ttl time.Duration
    now func() time.Time
    values sync.Map
}
```

方法：

```go
func New[K comparable, V any](ttl time.Duration) *Cache[K, V]
func (c *Cache[K, V]) Get(key K) (V, bool)
func (c *Cache[K, V]) Set(key K, value V)
func (c *Cache[K, V]) Delete(key K)
```

公共方法注释必须包含用途、参数说明、返回值说明和使用示例。TTL 仍由 auth application validators package 定义：

```go
const defaultTokenVersionLocalCacheTTL = time.Second
```

如果 TTL 小于等于 0，构造函数应回退到默认值，而不是创建永久本地缓存。

第一版不设计最大容量和后台清扫 goroutine。原因：

- token version 本地缓存 TTL 很短。
- 写入只发生在受保护请求校验路径。
- 不新增长期后台生命周期依赖，避免 application 层引入 workerpool 或 scheduler。

如果后续生产观察到高基数用户导致内存压力，再单独设计容量上限、随机采样清理或引入成熟 LRU/TTL cache。

## Validation Flow

推荐实现：

```go
func (v *tokenVersionValidator) ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
    currentVersion, err := v.Current(ctx, userID)
    if err != nil {
        return err
    }
    return commonauth.ValidateTokenVersion(tokenVersion, currentVersion)
}

func (v *tokenVersionValidator) Current(ctx context.Context, userID string) (int64, error) {
    if currentVersion, ok := v.cache.Get(userID); ok {
        return currentVersion, nil
    }

    value, err, _ := v.group.Do(userID, func() (any, error) {
        if currentVersion, ok := v.cache.Get(userID); ok {
            return currentVersion, nil
        }
        currentVersion, err := Current(ctx, v.users, v.sessions, userID)
        if err != nil {
            return int64(0), err
        }
        v.cache.Set(userID, currentVersion)
        return currentVersion, nil
    })
    if err != nil {
        return 0, err
    }
    return value.(int64), nil
}
```

`singleflight.Group.Do` 的 key 使用 user ID。closure 内二次检查本地缓存，避免多个 goroutine 等待时重复加载。

保留当前 package-level `Current(ctx, users, sessions, userID)` 函数，用作本地 cache miss 的加载函数，也避免影响 `sessions.Lifecycle.CurrentTokenVersion` 的现有调用方。

## Revocation Invalidation

本实例内 token version 撤销路径应主动删除本地缓存，避免额外等待 TTL。

当前撤销链路位于 auth application sessions lifecycle：

```text
RevokeAllUserSessions
  -> IncrementTokenVersion
  -> RevokeUserSessionsAtVersion
  -> CacheTokenVersion
  -> DeleteAllUserSessions
```

第一版推荐新增一个窄接口，只暴露本地失效能力：

```go
type TokenVersionLocalInvalidator interface {
    InvalidateTokenVersion(userID string)
}
```

`tokenVersionValidator` 实现：

```go
func (v *tokenVersionValidator) InvalidateTokenVersion(userID string) {
    v.cache.Delete(userID)
    v.group.Forget(userID)
}
```

在 Fx wiring 中把同一个 validator 实例同时作为：

- `commonauth.TokenVersionValidator`
- auth application 内部可选的 `TokenVersionLocalInvalidator`

然后由 session lifecycle 在完成 token version projection 刷新前后删除本地缓存。

如果为了避免扩大 Fx graph，第一版也可以只依赖短 TTL，不接入 invalidator；但实现时必须在测试和文档中明确最坏 stale window。当前方案要求接入本实例 invalidation，因为改动小且能降低安全窗口。

## Multi-instance Behavior

本变更不实现跨实例本地缓存失效广播。

多副本部署下，某个实例上的撤销请求只能同步失效本实例本地缓存；其他实例最多保留本地缓存 TTL 窗口。默认 TTL 应保持 `1s` 左右，使跨实例 stale window 可控。

未来如果需要更强实时性，应单独设计：

- Redis Pub/Sub 通知 user ID token version invalidation。
- 每个实例订阅后删除本地 cache 并 `singleflight.Forget(userID)`。
- 发布失败、订阅断线和补偿版本检查策略。

该能力与 RBAC policy refresh 类似，但属于 auth token version 撤销，不在本 change 混入。

## Error Semantics

本地缓存命中：

- 只返回 cached current version。
- version mismatch 由 `commonauth.ValidateTokenVersion` 产生现有错误。

本地 miss：

- 复用现有 `Current`。
- Redis unavailable 仍记录 `token version cache unavailable` 并返回错误。
- Redis cache miss 后 PostgreSQL 回源失败仍返回错误。
- Redis backfill 失败仍返回错误。

不缓存错误。错误缓存容易扩大外部依赖短暂抖动的影响，并可能改变当前中间件的 500 行为。

## Observability

第一版不新增 metrics 基础设施。

可以在测试中通过 stub 计数确认：

- 本地命中不会调用 Redis session store。
- 并发 miss 会被合并。
- TTL 过期后会重新加载。

如后续接入 metrics，建议计数项包括：

- `auth_token_version_l1_hit_total`
- `auth_token_version_l1_miss_total`
- `auth_token_version_singleflight_shared_total`
- `auth_token_version_l1_invalidate_total`

指标 label 不应包含 user ID。

## Redis Session Cleanup Guardrail

本变更不修改 Redis session 清理算法，但实现前后应保留以下判断：

- `DeleteAllUserSessions` 不使用 Redis `SCAN`。
- 默认 `max_active_sessions_per_user: 5` 下，单用户 session ZSET 应保持小集合。
- 若生产允许 `max_active_sessions_per_user: 0`，需要独立观测 `ZCARD`、Redis slowlog 和 purge task 耗时。

如果后续确认为大 key 问题，应单独设计限量 `ZREMRANGEBYSCORE`、后台慢清理或 `ZPOPMIN` 替代方案。

## Tests

建议测试分层：

### Common local cache tests

- `Get` miss。
- `Set` 后 TTL 内命中。
- TTL 过期后 miss。
- `Delete` 后 miss。
- 非正 TTL 回退默认值。
- 公共方法注释使用中文，并覆盖用途、参数、返回值和示例。

### Validator tests

- 首次校验 miss，会调用现有 loader 并写入本地 cache。
- 第二次同 user ID 校验命中本地 cache，不调用 Redis/session store stub。
- TTL 过期后重新调用 loader。
- 当前 version 大于 token version 时仍返回 `ErrTokenVersionMismatch`。
- loader 返回 Redis 或 DB 错误时不写入本地 cache。
- 同一 user ID 并发校验只触发一次 loader。
- 不同 user ID 并发校验互不合并。
- `InvalidateTokenVersion` 删除本地 cache 并让后续请求重新加载。

### Middleware boundary tests

已有 auth middleware 测试应保持：JWT 过期在 parse 阶段被拒绝，validator 不被调用。若当前没有显式断言，可补充一个 validator spy，确认 expired token 不触发 `ValidateTokenVersion`。

## Documentation

本变更新增 `common/runtime/localcache` runtime primitive。实现后需要确认 `docs/ARCHITECTURE.md` 和 `AGENTS.md` 的 common runtime 说明不会与该新增 primitive 冲突；如长期规则需要列举该 primitive，应同步补充。

实现完成后可以在本 change 的 tasks 中记录：

- 本地缓存 TTL。
- 是否已接入本实例 invalidation。
- 未实现跨实例 invalidation，最坏 stale window 为本地 TTL。
