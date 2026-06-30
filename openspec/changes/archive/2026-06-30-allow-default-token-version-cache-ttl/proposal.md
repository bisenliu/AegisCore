## Why

当前 `auth.token_version_cache_ttl` 的配置校验要求必须大于 0，但 Redis session store 的注释和逻辑又表达了非正数回退默认 TTL 的语义，导致配置契约与实现意图不一致。该差异会误导维护者，并使 token version 缓存行为在后续验证和运维配置中难以判断。

## What Changes

- 允许 `auth.token_version_cache_ttl` 使用 `0` 或负数表示“使用服务默认 TTL”。
- 保持显式正数 TTL 的现有行为不变。
- 明确默认 TTL 回退只影响 token version Redis 投影缓存，不改变 refresh session 生命周期、JWT 有效期或 token version 校验链路。
- 同步更新配置校验、相关注释和测试，避免 `make lint` 与 `make verify` 因配置语义或静态检查失败。
- 顺带修复已知 `fxgraph` Cobra `RunE` 未使用参数 lint 问题，并评估 RBAC 绑定测试中直接 `client.Schema.Create` 的替代路径。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `auth-session-management`: 调整 token version Redis 投影 TTL 的配置语义，允许非正数配置回退到默认 TTL。

## Impact

- 影响 `common/runtime/config` 中 auth 配置校验规则。
- 影响 `user-service/internal/features/auth/infrastructure/redis` 中 token version TTL 回退注释、逻辑和测试期望。
- 不影响外部 HTTP API、OpenAPI schema、数据库 schema、Ent migration 或 RBAC policy。
- 安全影响限定在缓存投影 TTL 选择；token version 仍以 PostgreSQL 当前值和 Redis 投影作为回源，旧 token 失效语义不变。
- 验证需要覆盖相关 Go 测试、`make lint`，并在实施完成后优先运行 `make verify`。
