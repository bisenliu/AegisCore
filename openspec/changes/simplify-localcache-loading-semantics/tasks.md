## 1. 收敛 common localcache API 与实现

- [X] 1.1 修改 `common/runtime/localcache/types.go` 和 `errors.go`：将 `Loader` 固定为 string key、`LoadingCache` 只保留 value 泛型，新增 `ErrInvalidated`，将 `Stats.Evicted` 改为 `CapacityEvictions`，并删除 `ErrClosed` 与旧 API 类型契约。
- [X] 1.2 重构 `common/runtime/localcache/cache.go`，只保留 `NewLoadingCache`、`Get`、`Invalidate`、`InvalidateAll`、`Name` 和 `Stats`；删除 `flightEntry`、per-key flight map、lifecycle/closed 状态、cleaner goroutine 和 eviction unsubscribe 生命周期。
- [X] 1.3 使用单个 `singleflight.Group.DoChan(key, loader)` 实现同 key miss 合并与不同 key 并行；loader context 使用 `context.WithTimeout(context.WithoutCancel(ctx), LoadTimeout)`，caller 取消后直接返回且不创建 drain goroutine。
- [X] 1.4 实现 `publishMu` 与 cache-wide `revision` 强失效算法：loader 前读取 revision，发布前比较 revision，一致时执行 `DeleteExpired` 与固定 TTL `Set`，失效时禁止返回/回填并最多透明重试一次，连续失效返回 `ErrInvalidated`。
- [X] 1.5 保持 `ttlcache.New[string, V]` 的固定 TTL、强制容量和命中不 touch 配置，确认未使用 `WithLoader`、`NewSuppressedLoader`、`Cache.Start` 或自建定时清理，并且 eviction callback 只累计 `EvictionReasonCapacityReached`。

## 2. 固定 localcache 并发、TTL、容量与统计行为

- [X] 2.1 重写 `common/runtime/localcache/cache_test.go` 的公开 API 测试，覆盖构造校验、固定 TTL 命中不续期、容量达到上限驱逐、loader error 不缓存及 `Stats` 请求级计数。
- [X] 2.2 增加 20 个同 key 并发 miss 仅执行一次 loader、不同 key 并行、caller 取消不取消共享 loader、loader timeout 返回 `context.DeadlineExceeded` 的确定性测试。
- [X] 2.3 增加受控 loader 并发测试，分别证明 `Invalidate` 与 `InvalidateAll` 返回后旧 loader 不返回、不回填，首次竞态透明重试取得新值，连续竞态返回 `ErrInvalidated`。
- [X] 2.4 增加 goroutine 稳定性与 eviction 口径测试，证明 canceled caller 不创建等待 loader 的额外 goroutine，显式失效和 TTL 到期不增加 `CapacityEvictions`，只有容量驱逐增加计数。

## 3. 迁移 localcache Prometheus collector

- [X] 3.1 修改 `common/runtime/observability/metrics/localcache.go`，从 `Stats.CapacityEvictions` 导出 `aegiscore_localcache_capacity_evictions_total`，同步 help 文本并删除旧 `aegiscore_localcache_evictions_total` 契约。
- [X] 3.2 更新 `common/runtime/observability/metrics` 的 collector、registry 和 fixture 测试，断言 requests、loads、capacity evictions、capacity 的名称、类型、label 和数值，且禁用 metrics 时不注册 collector。

## 4. 迁移 auth token-version cache 与撤销编排

- [X] 4.1 修改 `user-service/internal/features/auth/` 的消费侧 cache port、composition 与 validator，在 validator 边界使用 UUID 规范 string key 并直接调用 `Get`/`Invalidate`；删除 `localTokenVersionCacheAdapter` 及只为 `Close`、`ErrClosed` 存在的 resource closed 状态和 holder 分支。
- [X] 4.2 调整 auth Fx lifecycle，使 token-version localcache 不注册 close hook，同时继续显式管理 session purge pool 等真实主动资源，并保持共享 Redis/PostgreSQL 所有权边界。
- [X] 4.3 修改退出全部会话、已认证改密和强制改密的撤销编排，只保留 Redis token-version 投影更新前、更新后两次本地失效，删除 refresh sessions 删除后的第三次失效，并保留 Redis/session 失败的 `ErrSessionRevocationIncomplete` 错误链与 metrics。
- [X] 4.4 更新 auth 单元测试、生成 mocks 和 Redis adapter 测试以使用新 cache API，删除 closed/error 兼容断言；增加并发撤销测试，证明旧 token-version loader 不回填、不返回且 validator 在连续失效时 fail-closed。

## 5. 迁移 permission user-role cache

- [X] 5.1 修改 `user-service/internal/features/permission/infrastructure/casbin/user_role_cache.go` 及 resolver port，在 localcache 边界使用 `userID.String()`、`Get`、`Invalidate` 和 `InvalidateAll`，并继续在 loader 写入及 `RolesForUser` 返回边界调用 `cloneRoleIDs`。
- [X] 5.2 删除 `userRoleCacheGeneration`、generation token、stale generation error 和 feature-local retry/gate，确保 permission application/infrastructure 不再重复实现通用 revision 语义。
- [X] 5.3 收敛 permission Fx runtime 与 lifecycle，删除 user-role localcache 的 `Start`/`Close` view、hook 和 rollback 分支，只保留 watcher 等主动资源的幂等启停、错误聚合和共享资源所有权。
- [X] 5.4 更新 permission composition、Casbin resolver 和 policy 测试，覆盖 string key、slice 隔离、单用户/全量失效竞态、透明重试和连续失效 fail-closed，并删除 `ErrClosed` 与 stale generation 断言。
- [X] 5.5 保留 cache disabled direct resolver 的逐次 PostgreSQL 回源、独立 slice、`LoadSuccess`/`LoadError` 与 fail-closed 测试，确认强失效重构不改变禁用模式授权结果。

## 6. 同步观测资产与文档

- [X] 6.1 将 `deployments/observability/prometheus/user-service-alerts.yaml`、alert tests、metrics load 校验和相关脚本中的 localcache 查询改为 `aegiscore_localcache_capacity_evictions_total`，确保告警只表达容量压力而非 TTL 或显式失效。
- [X] 6.2 修改 `deployments/observability/grafana/user-service-overview.json` 的 panel、标题、说明和 PromQL，运行 `make compose-dashboard-generate` 更新 Compose provisioning dashboard，不直接编辑生成副本。
- [X] 6.3 更新 `docs/observability/user-service-runbook.md`、`deployments/observability/README.md` 及受影响测试说明，明确容量驱逐口径、强失效竞态风险与收敛条件，不保留旧指标名或旧 cache lifecycle 说明。
- [X] 6.4 再次运行 `make compose-dashboard-generate` 后检查相关 dashboard 无新增 diff，并运行 `make compose-dashboard-check` 证明源 dashboard 与 provisioning JSON 无 drift。

## 7. 定向验证与架构检查

- [X] 7.1 对修改的 Go 文件运行 `gofmt`，执行受影响 feature 的现有 `go generate` 入口并同步 mocks；再次运行生成命令并检查没有未预期 drift。
- [X] 7.2 在 `common/` 运行 `go test -race ./runtime/localcache ./runtime/observability/metrics`，仅在固定 TTL、容量、singleflight、取消、timeout、强失效、goroutine 和指标测试全部通过后完成本任务。
- [X] 7.3 在 `user-service/` 运行 `go test -race ./internal/features/auth/... ./internal/features/permission/...`，仅在 token-version 撤销和 user-role 授权并发测试全部通过后完成本任务。
- [X] 7.4 运行 `openspec validate simplify-localcache-loading-semantics` 和 `make user-service-architecture-lint`，确认四份 delta、common/feature 边界、代码注释语言和目录结构均符合主规格。
- [X] 7.5 检查 `git diff` 与全仓 `rg`，确认生产代码和观测资产不存在泛型 localcache key、`GetOrLoad`、`Delete`、`Clear`、localcache `Close`/`ErrClosed`、`flightEntry`、cleaner goroutine、`userRoleCacheGeneration` 或旧 `aegiscore_localcache_evictions_total` 残留，且没有 Ent schema、migration、HTTP API 或 OpenAPI 生成物变化。

## 8. 合并前门禁

- [X] 8.1 使用显式路径暂存本 change 的 OpenSpec artifacts、common localcache/metrics、auth、permission、测试、生成 mocks、观测资产和文档，检查 `git status --short`，确认暂存范围只包含本次预期变更。
- [X] 8.2 在全部预期变更已暂存后运行 `make lint`；仅在命令通过后将本任务标记完成。
- [X] 8.3 在全部预期变更已暂存后运行 `make verify`；仅在相关测试、生成检查和最终 `git diff --exit-code` 全部通过后将本任务及 change 标记完成。
