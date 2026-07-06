## Context

当前 Redis ping metrics collector 同时实现 Prometheus 标准 `Collect(ch)` 与项目扩展的 `CollectContext(ctx, ch)`。标准 Prometheus `Collector` 接口没有 context 参数，因此直接调用 `Collect` 或绕过项目 `metrics.Provider` 注册到原生 registry 时，只能依赖 collector timeout，不能感知 HTTP scrape request 取消。user-service 的真实 metrics 路由已经通过 `Provider.HTTPHandler` 和 `GatherContext` 传递 request context，本次变更需要把该语义固化为规格和测试，避免后续误用。

当前 token version validator 的本地缓存失效方法忽略 `localcache.Delete` 返回值。`localcache.Delete` 在 cache 已关闭等情况下会返回错误，但 `TokenVersionLocalInvalidator` 接口没有错误返回，导致会话撤销流程无法记录或传播本实例本地 token version cache 失效失败。该失效链路属于认证安全边界，需要可观测、可测试且可被调用方处理。

## Goals / Non-Goals

**Goals:**

- 明确 Redis metrics 的 request context 传播边界：只有经 `metrics.Provider.HTTPHandler` 或 `GatherContext` 采集时才承诺传播 scrape context。
- 保留 Redis collector 的 Prometheus 标准接口实现，但将 `Collect` 定义为 background context fallback，不承诺感知 scrape 取消。
- 修改 `TokenVersionLocalInvalidator` 为返回 `error`，并让会话撤销流程记录和返回本地失效失败。
- 增加测试覆盖 provider context 传播、标准 `Collect` fallback 语义，以及 token version 本地失效失败传播。

**Non-Goals:**

- 不改 Prometheus metric family、label、数值语义或 metrics HTTP 路由。
- 不引入新的 Prometheus registry、第三方依赖、全局 scrape context、goroutine-local context 或后台 worker。
- 不改变 PostgreSQL token version 递增、Redis token version 投影、refresh session 删除的主事实顺序。
- 不修改数据库 schema、OpenAPI、部署清单或 Grafana/Prometheus 资产。

## Decisions

1. Redis metrics context 通过 provider 传递，而不是修改标准 `Collect` 签名。

   Prometheus 标准接口无法携带 context，项目已有 `ContextCollector`、`contextCollectorWrapper`、`GatherContext` 和 `HTTPHandler`。最终方案是保持该架构，补充注释和测试，明确所有真实 HTTP scrape 必须经 provider 进入。备选方案是为 Redis collector 增加额外 registry wrapper 或隐式全局 context，但这会扩大 common runtime 复杂度并引入并发语义风险，因此不采用。

2. `Collect` 保持可用但只作为 fallback。

   `Collect` 继续调用 `CollectContext(context.Background(), ch)`，确保 collector 仍满足 `prometheus.Collector`。该 fallback 只受 collector timeout 约束，不承担 scrape cancellation 语义。备选方案是在 `Collect` 中 panic 或跳过探测以强制 provider 使用，但会破坏 Prometheus collector 的基本可注册性，因此不采用。

3. token version 本地失效错误必须显式返回。

   `TokenVersionLocalInvalidator.InvalidateTokenVersion` 改为返回 `error`，`TokenVersionValidator` 直接返回 `localcache.Delete` 的错误并包装上下文。会话撤销流程每次本地失效失败都记录日志并加入 `projectionErr`。备选方案是仅在 validator 内记录日志但保持无返回值，这会继续让调用方和测试无法判断撤销投影是否部分失败，因此不采用。

4. 本地失效失败按投影失败处理。

   PostgreSQL token version 递增仍是旧 access token 失效的主事实；本地 cache、Redis 投影和 refresh session 删除属于投影与清理链路。失效失败不回滚 PostgreSQL 主事实，但必须通过返回错误、日志和测试暴露。备选方案是本地失效失败直接中断后续 Redis 投影或 session 删除，但会降低最终一致修复机会，因此不采用。

## Risks / Trade-offs

- 本地 cache 已关闭时可能让撤销流程返回新的 projection error → 通过日志字段和错误包装标明是本地 cache 失效失败，调用方仍能区分主事实成功与投影失败。
- `TokenVersionLocalInvalidator` 签名变更会影响 mocks 和调用方 → 同步更新 `sessions` 测试 mock、validator 测试和 lifecycle 调用点，不保留旧接口适配。
- Redis `Collect` fallback 仍不能感知 request 取消 → 通过规格、注释和测试明确限制，真实 HTTP scrape 继续走 provider context 传播路径。
- Provider 使用 `gatherMu` 串行化 `GatherContext` 与 `gatherCtx` 临时状态 → 本次不改变并发模型，避免为单个 collector 引入更大运行时重构。

## Migration Plan

1. 更新 `common/runtime/observability/metrics` 的 Redis collector 注释和测试，验证 provider context 与标准 `Collect` fallback 语义。
2. 更新 `user-service/internal/features/auth/application/validators` 的 invalidator 接口和实现，使本地 cache 删除错误返回给调用方。
3. 更新 `user-service/internal/features/auth/application/sessions` 的本地失效调用，使失败记录日志并加入 `projectionErr`。
4. 更新 gomock 生成物或手写测试 mock 的签名及相关测试 expectation。
5. 运行相关包测试、架构 lint；无需执行 OpenAPI、Ent 或部署资产生成。

回滚策略：回滚本 change 的代码和 spec delta 即可恢复旧接口和旧行为；不涉及数据迁移、配置迁移或部署资源回滚。

## Open Questions

- 无。
