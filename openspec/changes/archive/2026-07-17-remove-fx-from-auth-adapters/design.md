## Context

auth feature 当前有三类 constructor 暴露 Fx/Dig 组合细节：PostgreSQL credential store 使用 `CredentialStoreParams` 嵌入 `fx.In` 并通过 `name:"primary_db"` 获取 Ent client；Redis session store 使用 `SessionStoreParams` 嵌入 `fx.In`，直接接收完整 `*config.Config` 和 named purge pool；HTTP controller 使用 `AuthControllerParams` 嵌入 `fx.In`。这些 constructor 位于 feature infrastructure 或 transport 包，却表达了服务级 DI metadata，使普通 Go 装配和测试需要理解 Fx 输入映射。

本 change 属于 `auth-session-management` 规格变更，但不改变认证运行时行为。变更集中在 auth adapter/controller 构造 API 与 user-service Fx composition 边界，HTTP API、Redis key、session TTL、token version、refresh rotation、错误码和安全语义保持不变。

## Goals / Non-Goals

**Goals:**

- 移除 auth PostgreSQL/Redis adapter 和 HTTP controller constructor 中的 `fx.In`、`fx.Out`、named tag 与 Dig metadata。
- 让 `CredentialStore`、`SessionStore` 和 `AuthController` 可通过普通参数或无 DI tag 的 feature-local Options 构造。
- 让 `SessionStore` 只接收 Redis client、key catalog、所需配置值、metrics 和 logger 等窄依赖，不再把完整 service config 当作 DI 容器。
- 在 `user-service/internal/features/auth/fx.go` 的 composition 边界完成 Fx named dependency 到普通 constructor 的适配。
- 更新生产调用点和测试，不保留旧 constructor 或兼容 wrapper。

**Non-Goals:**

- 不迁移 `SessionPurgePool` 或 token-version localcache 的关闭责任。
- 不删除 auth feature 的 Fx module，也不要求全服务移除 Fx。
- 不改变 token 签发、refresh rotation、session TTL、Redis key、错误码、HTTP API、OpenAPI 或安全语义。
- 不修改 Ent schema、Atlas migration、部署清单或观测资产。

## Decisions

1. auth adapter/controller constructor 使用普通参数或无 DI tag 的 Options。

   选择该方案是因为它把 feature 包的构造 API 恢复为普通 Go API，同时仍允许参数较多的 constructor 保持可读性。备选方案是保留 params struct 但只删除 `fx.In`；该方案虽然减少 Fx import，但如果字段仍承载 named tag 或完整 service config，仍会保留 composition 语义泄漏，因此不采用。

2. Fx named dependency 只在 `auth/fx.go` 中适配。

   `primary_db`、`cache_redis`、`auth_session_purge_pool` 仍是服务级装配事实，但这些 metadata 应留在 provider 边界。实现时可新增 feature-local provider 函数，从 Fx 输入结构接收 named dependency，再调用普通 constructor。备选方案是在 infrastructure 包内提供 Fx 专用 wrapper；该方案会把 DI 框架重新引入 adapter 包，不符合本 change 目标。

3. `SessionStore` constructor 接收窄设置而非完整 `*config.Config`。

   Redis session store 实际只需要应用名生成 key catalog、token version cache TTL、purge pool、metrics 和 Redis client。实现时优先在 composition 层创建 `KeyCatalog` 并提取 `cfg.Auth.TokenVersionCacheTTL`。备选方案是继续传入完整 config 但移除 Fx tag；该方案仍把 service config 当作隐式依赖容器，不符合 auth 规格中的最小依赖要求。

4. 不提供兼容 wrapper。

   这是明确的不兼容内部 API 清理，受影响调用点都在仓库内，可一次性迁移。保留旧 constructor 会延长 Fx/Dig metadata 的稳定面，并让规格要求变得含混。

## Risks / Trade-offs

- 构造 API 不兼容可能遗漏测试或 provider 调用点 → 使用 `rg` 覆盖 `CredentialStoreParams`、`SessionStoreParams`、`AuthControllerParams` 和 `New*` 调用点，并运行 auth infrastructure/transport 相关测试。
- Composition 层适配可能错误映射 named resource → 保持 named tag 只出现在 `auth/fx.go` 的私有 Fx params 中，并通过服务装配测试与 architecture lint 检查。
- `SessionStore` 从完整 config 改为窄 settings 时可能遗漏默认值逻辑 → 保留现有 `defaultTokenVersionCacheTTL` 等 fallback 语义，并在 session store 测试中覆盖零值或无效 TTL 行为。
- 移除 Fx import 后可能仍通过注释或 tags 残留 DI metadata → 使用指定 `rg` 命令对三个目标文件验证无 `go.uber.org/fx`、`go.uber.org/dig` 或 `fx.In/fx.Out` 残留。

## Migration Plan

1. 修改 auth PostgreSQL credential store constructor，移除 `CredentialStoreParams` 的 Fx metadata，并改造生产 provider 与测试调用点。
2. 修改 auth Redis session store constructor，使用普通参数或无 DI tag Options，显式传入 Redis client、key catalog、token version cache TTL、purge pool 和 metrics。
3. 修改 auth HTTP controller constructor，使用普通参数或无 DI tag Options，更新 routes/provider/router 相关测试。
4. 在 `user-service/internal/features/auth/fx.go` 中保留必要 Fx 输入结构，将 named resource 适配到普通 constructor。
5. 运行 auth 相关测试、指定 `rg` 检查、`openspec validate remove-fx-from-auth-adapters`、`make user-service-architecture-lint`，暂存预期变更后运行 `make lint` 与 `make verify`。

回滚方式：如果实施中发现装配风险不可接受，可在未发布前整体回滚本 change 的代码与规格 delta。发布后不需要数据迁移回滚，因为没有数据库、Redis key 或外部 API 变更。

## Open Questions

无。当前范围、非目标和验收命令已明确。
