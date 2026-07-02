## 1. OpenSpec 准备

- [x] 1.1 创建 `merge-auth-fx-prune-token-validator-func` change，并完成 proposal、design 和 shared-platform-primitives delta spec。
- [x] 1.2 确认 `openspec status --change merge-auth-fx-prune-token-validator-func` 达到 implementation-ready 状态。

## 2. Auth Fx provider 合并

- [x] 2.1 将 `user-service/internal/features/auth/localcache.go` 中的 imports、常量、`tokenVersionCacheParams`、`tokenVersionCacheResult` 和 `newTokenVersionLocalCache` 合并到 `fx.go`。
- [x] 2.2 删除 `user-service/internal/features/auth/localcache.go`，保持 `auth_token_version_cache` Fx name、`fx.Lifecycle` stop hook 和 `authvalidators.Current` 回源逻辑不变。
- [x] 2.3 对 auth feature 变更文件运行 `gofmt`。

## 3. 共享 middleware API 收紧

- [x] 3.1 删除 `common/http/middleware/auth.go` 中未使用的 `TokenVersionValidatorFunc` 类型和 `ValidateTokenVersion` 方法。
- [x] 3.2 清理 `common/http/middleware/auth.go` 中不再使用的 import，并运行 `gofmt`。
- [x] 3.3 使用全仓搜索确认 `TokenVersionValidatorFunc` 只剩 OpenSpec change 文档引用。

## 4. 验证与收尾

- [x] 4.1 运行 `go test ./internal/features/auth/...`。
- [x] 4.2 运行 `go test ./http/middleware`。
- [x] 4.3 暂存本次预期变更。
- [x] 4.4 运行 `make lint`。
- [x] 4.5 运行 `make verify`。
- [x] 4.6 检查最终 diff 和 OpenSpec 状态，确认 change artifacts 与代码变更完整。
