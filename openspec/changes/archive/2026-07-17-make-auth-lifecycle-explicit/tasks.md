## 1. Session Purge Pool 生命周期

- [x] 1.1 调整 `user-service/internal/features/auth/infrastructure/redis/session_purge_pool.go`，删除 `SessionPurgePoolParams`、`fx.Lifecycle`、`go.uber.org/fx` 和 ordering-only `cache_redis` 依赖。
- [x] 1.2 将 `NewSessionPurgePool` 改为只接收 logger 或最小 options 等真实依赖，并返回实现 `PurgeTaskPool` 且可显式 `Stop(context.Context) error` 的 pool。
- [x] 1.3 更新 session store 构造和调用点，确保 purge pool 仍作为 session store 的真实依赖参与删除任务提交与 drain。

## 2. Token Version Cache 生命周期

- [x] 2.1 在 auth feature 内为 token-version localcache 构造结果定义显式关闭契约，暴露 cache/validator、stats 和幂等 `Close`。
- [x] 2.2 更新 enabled localcache 分支，使 `Close` 只关闭本地缓存且可重复调用。
- [x] 2.3 更新 disabled/direct 分支，使其返回一致的 no-op `Close`，并保持现有回源、stats、TTL/容量和 metrics 语义不变。
- [x] 2.4 保持 `TokenVersionValidator` 与 invalidator 的认证和撤销语义不变，关闭后的 `localcache.ErrClosed` 仍不得破坏安全撤销路径。

## 3. Fx Module 装配

- [x] 3.1 更新 `user-service/internal/features/auth/fx.go` 中 purge pool provider，使用新 API 构造普通 Go 资源并在 module 边界登记 `OnStop`。
- [x] 3.2 更新 `newTokenVersionLocalCache` 或等价 provider，把 token-version cache 的 `Close` 注册到 Fx lifecycle，同时继续输出 `auth_token_version_cache` 的 cache 与 stats。
- [x] 3.3 确认 `go.uber.org/fx` 只保留在 auth 装配层或测试中，auth Redis infrastructure 正式代码不再导入 Fx/Dig 或声明 `name:"cache_redis"` ordering-only dependency。

## 4. 测试

- [x] 4.1 更新 `user-service/internal/features/auth/infrastructure/redis` 相关测试，适配新的 `NewSessionPurgePool` API。
- [x] 4.2 增加或更新测试证明 session purge pool `Stop` 幂等、尊重超时、可 drain 已提交任务，且重复停止不 panic、不泄漏 goroutine。
- [x] 4.3 增加或更新测试证明 purge pool 停止完成后才允许共享 Redis client 关闭，且 pool 停止不会关闭 Redis client。
- [x] 4.4 增加或更新 token-version cache 测试，覆盖 enabled、disabled/direct 模式的幂等 `Close`、stats 暴露和关闭后安全行为。
- [x] 4.5 更新 `user-service/internal/features/auth/fx_test.go`，验证 Fx module 登记新资源 API 与关闭 hook，且不会把 Fx 依赖重新引入 Redis infrastructure。

## 5. 验证

- [x] 5.1 运行 `cd user-service && go test ./internal/features/auth/... -count=1` 并修复失败。
- [x] 5.2 运行 `rg -n 'go\.uber\.org/(fx|dig)|fx\.Lifecycle|name:"cache_redis"' user-service/internal/features/auth/infrastructure/redis --glob '*.go' --glob '!**/*_test.go'`，确认无输出。
- [x] 5.3 运行 `openspec validate make-auth-lifecycle-explicit` 并修复失败。
- [x] 5.4 运行 `make user-service-architecture-lint` 并修复失败。
- [x] 5.5 暂存本次预期代码、规格和文档变更后运行 `make lint`，并修复失败。
- [x] 5.6 保持本次预期变更已暂存后运行 `make verify`，并修复失败或记录无法完成的环境原因。
