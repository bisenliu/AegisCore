## Context

`auth.token_version_cache_ttl` 当前在 `common/runtime/config` 中被校验为正数，但 `user-service/internal/features/auth/infrastructure/redis/session_store.go` 在写入 Redis token version 投影时已把非正数 TTL 解释为使用 `defaultTokenVersionCacheTTL`。这使配置加载与 auth Redis adapter 的契约不一致：配置层拒绝了实现层声明支持的默认值语义。

该变更横跨 `common` 配置校验和 `user-service` auth Redis adapter，但不引入新的外部依赖，不改变 HTTP API、OpenAPI、数据库 schema、migration、部署清单或观测资产。安全边界仍由 JWT 校验、token version validator、Redis 投影和 PostgreSQL 当前值共同保证。

## Goals / Non-Goals

**Goals:**

- 允许 `auth.token_version_cache_ttl <= 0` 通过配置校验，并在写入 Redis token version 投影时稳定回退到默认 TTL。
- 保持 `auth.token_version_cache_ttl > 0` 的显式 TTL 行为不变。
- 将注释、配置校验和测试期望统一为同一语义。
- 修复当前阻塞 `make lint` 的 `fxgraph` Cobra `RunE` 未使用参数问题。
- 评估并尽量移除 RBAC 绑定测试中直接 `client.Schema.Create` 的表创建路径，避免测试路径偏离 Atlas migration。

**Non-Goals:**

- 不改变 access token、refresh token、refresh session 或 password change token 的有效期语义。
- 不调整 token version 校验链路中的本地 loading cache、Redis 投影、PostgreSQL 回源顺序。
- 不新增配置项、环境变量、HTTP API、数据库字段或 migration。
- 不重构角色 HTTP controller；该重构可在不改变稳定行为时作为独立实现质量任务处理。

## Decisions

1. 配置层允许非正数 token version Redis 投影 TTL。

   选择：将 `auth.token_version_cache_ttl` 的校验从“必须为正数”改为“允许任意 duration，由 auth Redis adapter 对非正数应用默认 TTL”。

   理由：现有 adapter 已具备默认 TTL 回退，保留该路径可以避免把默认行为分散到配置 loader、provider 和 Redis adapter 多处。

   备选方案：删除 Redis adapter 的 fallback 并保持严格正数配置。该方案改动更小，但会让默认 TTL 语义消失，并要求所有部署显式配置该值，不符合当前代码注释表达的意图。

2. 默认 TTL 只在 Redis token version 投影写入点生效。

   选择：不在 `common/runtime/config` 中把非正数 duration 改写为默认值，避免 `common` 固化 user-service 业务默认值。

   理由：`common/runtime/config` 是跨服务配置 primitive，不应承载 auth token version 的业务默认 TTL；默认值归属 auth Redis adapter 更符合现有边界。

   备选方案：在配置解析或校验后统一归一化 TTL。该方案会让所有配置消费者看到正数 TTL，但会把业务语义上移到共享配置层。

3. 测试覆盖配置校验与 Redis TTL 回退边界。

   选择：新增或调整测试，覆盖正数 TTL 保持原值、`0` 和负数 TTL 回退默认值、无效 JWT TTL 仍被拒绝。

   理由：该变更的风险主要来自配置接受范围扩大，测试需要证明放宽只作用于 token version Redis 投影 TTL。

   备选方案：只修正文档和注释。该方案无法防止后续配置校验再次收紧。

4. 质量修复与行为变更分层处理。

   选择：在同一实施中修复阻塞 `make lint` 的 `_ *cobra.Command` 参数问题，并把测试 harness 收敛列为验证任务；角色 controller 拆分不作为本 change 的必须行为。

   理由：lint 失败直接影响 `make verify`，需要与本次 apply 一起收敛；controller 文件拆分不改变规格行为，且可能扩大 review 面。

   备选方案：将所有用户列出的事项都纳入本 change。该方案会混合行为变更、测试基础设施和纯重构，增加回归风险。

## Risks / Trade-offs

- [Risk] 接受负数 TTL 可能被误解为永久缓存。→ Mitigation：保留并测试 Redis adapter 的默认 TTL 回退，注释明确非正数不会创建永久缓存项。
- [Risk] `common/runtime/config` 放宽校验后其他调用方误用该字段。→ Mitigation：该字段仍位于 auth 配置结构，实际 TTL 解释留在 auth session store，并通过 auth 测试覆盖。
- [Risk] 测试中移除 `client.Schema.Create` 可能暴露 migration/test harness 缺口。→ Mitigation：优先使用现有 migration/test harness；若现有工具不足，在 tasks 中记录后续缺口，不引入运行时代码中的 schema auto create。
- [Risk] 该变更可能被认为降低配置严格性。→ Mitigation：只放宽 token version Redis 投影 TTL，JWT TTL、HTTP timeout、local cache TTL 等仍保持正数校验。

## Migration Plan

- 部署前无需数据库 migration 或 OpenAPI 生成。
- 已配置正数 `auth.token_version_cache_ttl` 的环境行为不变。
- 配置为 `0` 或负数的环境会从启动失败变为启动成功，并在缓存写入时使用默认 TTL。
- 回滚方式：恢复配置校验为正数要求，并确保部署环境显式设置正数 `auth.token_version_cache_ttl`。

## Open Questions

- 无待决问题。
