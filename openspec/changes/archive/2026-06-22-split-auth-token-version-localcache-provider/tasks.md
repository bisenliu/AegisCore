## 1. Provider 文件拆分

- [x] 1.1 新增 `user-service/internal/features/auth/localcache.go`，迁入 `authTokenVersionCacheName`、`tokenVersionCacheParams`、`tokenVersionCacheResult` 和 `newTokenVersionLocalCache`。
- [x] 1.2 从 `user-service/internal/features/auth/fx.go` 删除 token version localcache provider 类型和构造函数，只保留 `newTokenVersionLocalCache` 在 provider 列表中的引用。
- [x] 1.3 调整 `fx.go` 与 `localcache.go` imports，并运行 `gofmt`，确保两个文件职责清晰且可编译。

## 2. 行为保持检查

- [x] 2.1 确认 `newTokenVersionLocalCache` 仍读取 `local_cache.auth_token_version`，缺少配置时仍返回 `local_cache.auth_token_version is required`。
- [x] 2.2 确认 `tokenVersionCacheResult` 仍以 `name:"auth_token_version_cache"` 暴露 `*localcache.Cache[string, int64]` 和 `localcache.StatsSource`。
- [x] 2.3 确认 localcache 构造参数、`authvalidators.Current` 回源函数和 `fx.Lifecycle` `OnStop` 关闭 hook 与拆分前一致。
- [x] 2.4 使用 `rg "tokenVersionCacheParams|tokenVersionCacheResult|func newTokenVersionLocalCache" user-service/internal/features/auth/fx.go` 确认 `fx.go` 不再承载 localcache provider 实现。

## 3. 验证与收尾

- [x] 3.1 在 `user-service` 模块运行 `go test ./internal/features/auth/...`，确认 auth feature 相关测试通过。
- [x] 3.2 检查 `git diff`，确认代码改动只包含 `user-service/internal/features/auth/fx.go`、`user-service/internal/features/auth/localcache.go` 和本 change artifacts。
- [x] 3.3 实现完成后将对应 tasks checkbox 更新为 `- [x]`，并确认 `openspec status --change split-auth-token-version-localcache-provider` 状态正常。
