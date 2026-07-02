## Why

当前 token version validator 测试使用手写 user store 和 token cache 替身，和包内已有 gomock 生成物并存，导致测试表达方式不一致，也降低了对 port 交互、cache miss、singleflight 合并和失效重载行为的可读性与可维护性。

本变更通过统一使用已有 gomock 生成物，使 validator 测试直接以 expectation 表达依赖交互，同时保留真实 `localcache` 实例来继续验证本地缓存行为。

## What Changes

- 将 `user-service/internal/features/auth/application/validators` 包内 token version validator 测试的手写测试替身替换为已有 gomock mock。
- 移除 `tokenVersionUserTestStore` 和 `tokenVersionSessionTestStore`，不保留兼容测试替身。
- 使用 `mock_generate.go` 已生成的 `UserTokenVersionStore` 与 `TokenVersionCache` mock 表达 user store 和 Redis token version cache expectation。
- 保留真实 `localcache` 实例，继续覆盖本地缓存命中、Redis cache miss、PostgreSQL 回源、singleflight 合并和失效重载行为。
- 并发测试通过 `DoAndReturn`、channel、mutex 或 atomic 计数在测试内部表达并发控制，覆盖 singleflight 合并、按用户隔离和失效重载。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 明确 token version validator 的测试应覆盖本地缓存、Redis 投影、PostgreSQL 回源、singleflight 合并、按用户隔离和失效重载行为，并使用已有 gomock 生成物表达依赖交互；生产行为不变。

## Impact

- 影响代码范围：仅 `user-service/internal/features/auth/application/validators` 包内测试。
- 不影响生产代码、auth middleware、`localcache` primitive、Redis integration/miniredis 测试、HTTP API、OpenAPI、数据库 schema、部署资产或共享契约。
- 验证要求：`make user-service-generate` 后无 mockgen drift，`cd user-service && go test ./internal/features/auth/application/validators` 通过，`make user-service-architecture-lint` 通过。
