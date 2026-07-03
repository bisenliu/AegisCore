## Context

auth feature 根目录当前包含 `fx.go`、`localcache.go`、`metrics.go` 及对应测试。`localcache.go` 中的 `newTokenVersionLocalCache` 是 auth Fx module 注册的 provider，负责读取 `local_cache.auth_token_version`、构造有界 loading cache、暴露 `auth_token_version_cache` 命名依赖，并在 Fx lifecycle stop 时关闭缓存。

`common/http/middleware/auth.go` 当前额外导出 `TokenVersionValidatorFunc`，用于把函数适配为 `common/security/auth.TokenVersionValidator`。仓库内调用方均直接依赖 `common/security/auth.TokenVersionValidator` 或具体实现，没有消费者使用该适配器。

## Goals / Non-Goals

**Goals:**

- 将 auth token version localcache provider 实现合并回 `user-service/internal/features/auth/fx.go`，使 auth 根目录中的 Fx provider 集中呈现。
- 删除 `user-service/internal/features/auth/localcache.go`，不改变 provider 函数名、Fx name tag、生命周期 hook、配置 key 或回源逻辑。
- 从 `common/http/middleware` 删除未使用的 `TokenVersionValidatorFunc` 导出 API。
- 保持 `common/security/auth.TokenVersionValidator` 作为受保护路由 token version 校验的稳定接口。

**Non-Goals:**

- 不改变 JWT 解析、token version mismatch、refresh session、强制改密、登出或 RBAC 授权行为。
- 不修改 `common/security/auth.TokenVersionValidator`、`authvalidators.NewCachingValidator` 或 metrics wrapper 的接口语义。
- 不修改 HTTP API、OpenAPI、Ent schema、Atlas migration、部署清单、Prometheus 指标或 Grafana dashboard。
- 不新增 common helper、shared 包、integration adapter 或测试专用生产接口。

## Decisions

1. 将 `localcache.go` 的 provider 代码原样并入 `fx.go`。

   - 理由：本次最终方案要求所有 auth Fx provider 函数集中到 `fx.go`；同包合并不会改变 Go 包 API，也不会影响 Fx 依赖图。
   - 备选：重命名为 `fx_localcache.go`。该方式能减少歧义但不满足本次集中 provider 的目标。

2. 保留 `newTokenVersionLocalCache`、`tokenVersionCacheParams` 和 `tokenVersionCacheResult` 名称。

   - 理由：本次只调整文件归属，保留符号名可以降低误改注入契约和测试引用的风险。
   - 备选：重命名 provider 或参数类型。该方式会扩大 diff，且不会带来运行时收益。

3. 直接删除 `TokenVersionValidatorFunc`，不先标记 deprecated。

   - 理由：仓库内没有消费者，且该类型只是函数到既有接口的薄适配器，没有独立稳定语义；保留会扩大共享 middleware API 表面。
   - 备选：先添加 deprecated 注释。该方式适合已知外部消费者迁移期，但当前 change 明确采用不兼容清理。

## Risks / Trade-offs

- [Risk] 合并 provider 时遗漏 import、Fx tag 或 lifecycle hook，导致 auth feature 编译或装配失败。Mitigation：原样迁移实现并运行 `go test ./internal/features/auth/...`。
- [Risk] 删除导出适配器影响仓库外调用方。Mitigation：在 proposal 和 spec 中标记 API 收缩，迁移路径是提供实现 `common/security/auth.TokenVersionValidator` 的具体类型或调用方自有函数适配器。
- [Risk] `fx.go` 文件变长，模块声明与 provider 细节重新集中。Mitigation：保持 provider 代码块相邻且不引入额外业务逻辑，后续如需要可按新的命名规范再拆分。

## Migration Plan

1. 将 `localcache.go` 中的 import、常量、`fx.In`/`fx.Out` 类型和 `newTokenVersionLocalCache` 实现合并到 `fx.go`。
2. 删除 `localcache.go`。
3. 删除 `common/http/middleware/auth.go` 中的 `TokenVersionValidatorFunc` 类型和方法，并清理不再使用的 import。
4. 运行 `gofmt` 于变更 Go 文件。
5. 运行 `go test ./internal/features/auth/...` 和 `go test ./http/middleware`。
6. 回滚方式：恢复 `localcache.go` 并从 `fx.go` 移出 provider 实现；恢复 `TokenVersionValidatorFunc` 类型和方法。

## Open Questions

- 无。
