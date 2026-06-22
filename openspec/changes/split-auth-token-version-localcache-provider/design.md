## Context

`user-service/internal/features/auth/fx.go` 当前负责声明 `feature-auth` Fx 模块、列出 provider、提供 session lifecycle adapter、构造 token version validator，并内联 token version localcache 的参数结构、返回结构和构造函数。

token version localcache provider 本身包含配置实例读取、`common/runtime/localcache` 构造、`authvalidators.Current` 回源函数、`localcache.StatsSource` 导出和 `fx.Lifecycle` 关闭 hook。这些逻辑是 auth feature 的运行时装配细节，但不需要留在模块声明文件中。

本 change 只调整 auth feature 内部文件归属；不跨模块移动代码，不改变 `common` primitive，不触碰 HTTP API、数据库 migration、OpenAPI 生成物、部署清单、观测资产或安全边界。

## Goals / Non-Goals

**Goals:**

- 新增 `user-service/internal/features/auth/localcache.go`，承载 token version localcache provider 的参数、结果和构造函数。
- 让 `user-service/internal/features/auth/fx.go` 只保留模块声明、provider 列表、auth session lifecycle adapter 和 token version validator adapter。
- 保持 `authTokenVersionCacheName`、`auth_token_version_cache` Fx name、`localcache.StatsSource` 输出、`fx.Lifecycle` 关闭 hook、localcache 参数映射和 `authvalidators.Current` 回源逻辑不变。
- 保持 `TestNewTokenVersionLocalCacheRequiresConfigInstance` 等 auth 测试继续覆盖 provider 行为。

**Non-Goals:**

- 不改变 `TokenVersionValidator`、`authvalidators.Current` 或 token version mismatch 处理。
- 不改变 `config.LocalCacheConfig`、`local_cache.auth_token_version` 配置结构或默认配置。
- 不改变 auth HTTP API、OpenAPI、数据库 schema、migration、Redis key schema、metrics 名称或 Fx 注入契约。
- 不把 auth 业务语义移动到 `common`、`internal/shared`、`internal/integration` 或 provider 外的新共享包。

## Decisions

1. 将 provider 代码拆到同包 `localcache.go`，而不是新增子包。

   - 理由：`newTokenVersionLocalCache` 是 auth feature 的 Fx provider，实现需要访问同包常量和 application port；留在 `auth` 包内可以避免暴露额外 public API，也不会引入 import cycle。
   - 备选：创建 `internal/features/auth/infrastructure/localcache` 子包。该方案会把 Fx provider 的 `fx.In`/`fx.Out` 装配类型放进基础设施子包，并迫使当前同包测试或 provider 列表跨包引用，收益不足。

2. 保留原有类型名和函数名。

   - 理由：该 change 是物理拆分，不是 API 或语义重命名；保留 `tokenVersionCacheParams`、`tokenVersionCacheResult` 和 `newTokenVersionLocalCache` 可让测试、provider 列表和错误信息保持稳定。
   - 备选：重命名为更通用的 localcache provider 名称。该方式会制造无关 diff，并提高误改 Fx 注入契约的风险。

3. 只从 `fx.go` 移除 localcache 直接依赖的 import。

   - 理由：拆分后 `context`、`fmt` 和 `localcache` 只属于 `localcache.go`；`fx.go` 仍保留 `config`、`commonauth` 和 auth application 依赖，用于轻量 adapter。
   - 备选：继续在 `fx.go` 保留空转 import 或注释。Go 编译不会允许未使用 import，也会让模块声明文件继续显得承担缓存实现职责。

4. 验证聚焦 auth feature 包。

   - 理由：行为不变且影响范围限定在 auth feature 内部文件归属，`go test ./internal/features/auth/...` 能覆盖 provider 编译、missing config 测试、validator 缓存、session lifecycle、Redis/PostgreSQL adapter 相关测试。
   - 备选：运行全仓 `make verify`。可作为合并前更大范围验证，但本 change 的直接反馈优先使用 auth feature 测试。

## Risks / Trade-offs

- [Risk] 拆分 import 时误删 `fx.go` 中仍被 validator adapter 或 session lifecycle adapter 使用的依赖 -> Mitigation: 运行 `go test ./internal/features/auth/...` 覆盖编译。
- [Risk] 移动 provider 时误改 `fx.Out` name tag，导致 metrics provider 或 validator 注入失败 -> Mitigation: 保持 `name:"auth_token_version_cache"` 原样，并通过 auth feature 测试和 Fx 编译路径验证。
- [Risk] 生命周期 hook 或 `StatsSource` 返回值在移动时遗漏 -> Mitigation: 原样迁移 `params.Lifecycle.Append` 和 `tokenVersionCacheResult{Cache: cache, Stats: cache}`，并在 code review 中重点检查。
- [Risk] specs artifact 使用 `MODIFIED Requirements` 但本次不改变稳定行为 -> Mitigation: delta spec 完整复述现有 `auth-session-management` token version requirement，不引入新语义。

## Migration Plan

1. 新增 `user-service/internal/features/auth/localcache.go`，迁入 token version localcache provider 相关代码。
2. 从 `user-service/internal/features/auth/fx.go` 删除 provider 类型和构造函数，保留 provider 列表中的 `newTokenVersionLocalCache` 引用。
3. 执行 `gofmt` 于变更 Go 文件。
4. 在 `user-service` 模块运行 `go test ./internal/features/auth/...`。
5. 检查 diff，确认只包含目标 Go 文件和本 change artifacts。

回滚方式：恢复 `newTokenVersionLocalCache` 与相关类型到 `fx.go` 并删除 `localcache.go`。该 change 不涉及数据迁移、配置迁移或部署顺序调整。

## Open Questions

- 无。
