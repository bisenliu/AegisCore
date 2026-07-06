## Why

Redis metrics collector 已经提供 `CollectContext(ctx, ch)`，但标准 Prometheus `Collect(ch)` 无法携带 scrape context，容易让后续使用方误以为直接注册到原生 registry 也能感知 scrape 取消。认证 token version 本地缓存失效失败当前被忽略，撤销会话或刷新 token version 投影时缺少日志和错误传播，降低安全相关问题的排障能力。

## What Changes

- 明确 Redis ping metrics 的 scrape 取消语义：真实 HTTP scrape 必须通过 `metrics.Provider.HTTPHandler` 或 `GatherContext` 传递 request context；标准 `Collect` 仅作为 Prometheus 接口 fallback，使用 background context 与 collector timeout。
- **BREAKING** 修改 token version 本地缓存失效接口：`TokenVersionLocalInvalidator.InvalidateTokenVersion` 返回 `error`，不再吞掉本地 cache 删除失败。
- 会话撤销流程在每次本地 token version cache 失效失败时记录日志，并将失败纳入投影错误返回，避免静默留下陈旧本地 token version。
- 增加覆盖 Redis collector context 语义与 token version 本地缓存失效失败传播的测试。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `runtime-observability`: 明确 Redis metrics collector 在 provider scrape context 与标准 `Collect` fallback 下的取消语义。
- `auth-session-management`: 要求 token version 本地缓存失效失败可观测且可被调用方处理。

## Impact

- 影响 `common/runtime/observability/metrics` 中 Redis collector 与 provider 相关文档、注释或测试，不改变 Prometheus 指标名称、label、HTTP 路由或 OpenAPI。
- 影响 `user-service/internal/features/auth/application/validators` 的 invalidator 接口签名和实现。
- 影响 `user-service/internal/features/auth/application/sessions` 的会话撤销错误处理与 gomock 测试桩。
- 不涉及数据库 schema、Ent 生成物、Atlas migration、OpenAPI 生成物、部署资产或外部 API 契约。
