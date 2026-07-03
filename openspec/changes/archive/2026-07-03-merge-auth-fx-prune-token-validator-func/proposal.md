## Why

`user-service/internal/features/auth/localcache.go` 承载 Fx provider 装配逻辑但文件名没有体现 Fx 语义，容易让 auth 根目录的职责边界变得含糊。`common/http/middleware.TokenVersionValidatorFunc` 是导出的函数适配器，但当前仓库没有消费者，继续保留会增加共享 HTTP middleware API 噪音。

## What Changes

- 将 auth token version localcache provider 相关常量、`fx.In`/`fx.Out` 类型和 `newTokenVersionLocalCache` 合并回 `user-service/internal/features/auth/fx.go`。
- 删除 `user-service/internal/features/auth/localcache.go`，保持 Fx provider、命名注入、生命周期 hook、localcache 构造和回源逻辑不变。
- **BREAKING**：从 `common/http/middleware` 删除未使用的导出类型 `TokenVersionValidatorFunc` 及其 `ValidateTokenVersion` 方法。
- 保持 `common/security/auth.TokenVersionValidator` 作为 token version 校验的稳定接口，调用方继续直接传入具体实现。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 收紧共享 HTTP middleware 的导出 API，移除没有仓库内消费者且只包装既有接口的 token version validator 函数适配器。

## Impact

- 影响代码范围：`user-service/internal/features/auth/fx.go`、`user-service/internal/features/auth/localcache.go`、`common/http/middleware/auth.go`。
- 共享 API 影响：删除 `common/http/middleware.TokenVersionValidatorFunc`，仓库内调用方不受影响；仓库外如直接引用该导出类型，需要改为提供实现 `common/security/auth.TokenVersionValidator` 的具体类型或自定义适配器。
- 不影响 auth HTTP API、JWT 解析、token version 校验语义、localcache 配置 key、Fx 注入 name、数据库 schema、OpenAPI、部署资产或观测指标名称。
- 验证范围：优先运行 `go test ./internal/features/auth/...` 和 `go test ./http/middleware`。
