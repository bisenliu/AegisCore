## Why

`user-service/internal/features/auth/fx.go` 同时承载认证模块装配和 token version localcache provider 细节，使 Fx 模块声明被缓存构造、生命周期 hook 和回源逻辑稀释。

将 token version localcache provider 拆到独立文件，可以让 `fx.go` 更专注于模块声明、provider 列表和轻量 adapter，同时保持 token version 校验链路的运行时行为不变。

## What Changes

- 新增 `user-service/internal/features/auth/localcache.go`，承载 `tokenVersionCacheParams`、`tokenVersionCacheResult` 和 `newTokenVersionLocalCache`。
- 从 `user-service/internal/features/auth/fx.go` 移出 token version localcache provider 相关类型和构造函数。
- 保留 `auth_token_version_cache` Fx name、`localcache.StatsSource` 暴露、`fx.Lifecycle` 关闭 hook、localcache 配置读取和 `authvalidators.Current` 回源逻辑。
- 保持 token version validator、localcache 配置结构、auth HTTP API、OpenAPI 和数据库 schema 不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 整理 token version localcache provider 的文件归属，保持 token version 校验链路、缓存、失效、StatsSource 和回源需求语义不变。

## Impact

- 影响代码范围：`user-service/internal/features/auth/fx.go`、`user-service/internal/features/auth/localcache.go`。
- 相关测试范围：auth feature 相关 Go 测试，优先运行 `go test ./internal/features/auth/...` 于 `user-service` 模块。
- 不影响 auth HTTP API、OpenAPI、数据库 schema、migration、部署资产、观测指标名称、安全契约或共享契约。
