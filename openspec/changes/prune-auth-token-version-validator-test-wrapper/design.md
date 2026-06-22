## Context

`user-service/internal/features/auth/application/validators/token_version_validator_test.go` 覆盖 token version validator 的本地缓存、Redis miss 后回源、loader 错误不缓存、singleflight 合并、不同用户隔离和本地失效语义。

当前测试 helper `newTestTokenVersionValidator` 返回 `testTokenVersionValidator` wrapper。该 wrapper 嵌入 `*TokenVersionValidator`，同时持有 `cache *localcache.Cache[string, int64]` 字段，但测试只调用 validator 方法，不读取 cache 字段。cache 生命周期已经由 helper 内的 `t.Cleanup(cache.Close)` 管理，因此 wrapper 没有提供额外测试价值。

本 change 只影响认证 application validators 测试文件；不影响 Go 生产代码、HTTP API、数据库 migration、OpenAPI 生成物、部署清单、观测资产或安全边界。

## Goals / Non-Goals

**Goals:**

- 删除 `testTokenVersionValidator` 测试 wrapper 类型。
- 让 `newTestTokenVersionValidator` 直接返回 `*TokenVersionValidator`。
- 保留 helper 内 local cache 构造和 `t.Cleanup(cache.Close)`，确保测试生命周期不变。
- 保持现有测试语义、断言和覆盖面不变。
- 运行 auth validators 相关测试验证行为没有 drift。

**Non-Goals:**

- 不修改 `TokenVersionValidator` 生产代码。
- 不调整 `common/runtime/localcache` 行为或配置。
- 不改变 token version 校验、失效、Redis 投影或 PostgreSQL 回源语义。
- 不修改 API、schema、OpenAPI、部署或观测资产。

## Decisions

1. 直接返回 `*TokenVersionValidator`，而不是保留 wrapper 或新增接口。

   - 理由：测试只需要 validator 的公开方法，直接返回生产类型能减少间接层，并让 helper 意图与生产 API 对齐。
   - 备选：保留 wrapper 但删除 cache 字段。该方式仍留下一个没有额外语义的测试专用类型，不如直接返回生产类型清晰。

2. cache 保留为 `newTestTokenVersionValidator` 局部变量。

   - 理由：cache 只用于构造 `NewCachingValidator(cache)` 并注册 cleanup，局部变量已经覆盖生命周期管理需求。
   - 备选：把 cache 暴露给测试调用方。当前没有断言需要直接检查 cache 内部状态，暴露会鼓励测试耦合 localcache 实现细节。

3. 不改动测试场景和断言。

   - 理由：本 change 是测试结构整理，验收点是 helper 形态简化且行为保持一致；调整覆盖面会扩大变更风险。
   - 备选：重命名或拆分测试。当前测试名称已准确表达场景，重命名会制造无关 diff。

## Risks / Trade-offs

- [Risk] 删除 wrapper 后若未来测试需要直接检查 cache，需要重新扩展 helper 返回值 → Mitigation: 当前不暴露未使用状态；未来出现真实断言需求时再引入专门 helper 或返回额外值。
- [Risk] 修改 helper 返回类型可能影响同文件调用点编译 → Mitigation: 该 helper 仅在本测试文件内使用，运行 auth validators 包测试覆盖编译和行为。
- [Risk] OpenSpec schema 要求 specs artifact，但本次不改变稳定需求 → Mitigation: delta spec 完整复述 `auth-session-management` 中现有 token version 策略 requirement，不引入新语义。

## Migration Plan

1. 修改 `token_version_validator_test.go`，删除 `testTokenVersionValidator` 类型。
2. 将 `newTestTokenVersionValidator` 返回类型改为 `*TokenVersionValidator`，返回 `NewCachingValidator(cache)`。
3. 保留 `t.Cleanup(cache.Close)` 和现有 localcache 配置。
4. 运行 `go test ./internal/features/auth/application/validators`。

回滚方式：本 change 只修改测试文件和 OpenSpec change artifacts。如测试 helper 整理出现问题，可回退该测试文件改动，不涉及数据、配置或运行时迁移。

## Open Questions

- 无。
