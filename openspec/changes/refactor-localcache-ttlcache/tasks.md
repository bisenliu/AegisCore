## 1. 重构共享 Loading Cache

- [x] 1.1 更新 `common/go.mod` 与 `common/go.sum`，引入 `github.com/jellydator/ttlcache/v3`、删除 Ristretto，并确认依赖整理没有引入无关模块变化
- [x] 1.2 重写 `common/runtime/localcache/types.go` 和 `errors.go`，提供非泛型 `Config`、`LoadingCache` 最小公开契约与新 `Stats`，删除 `Cache`、`New`、`CloneFunc`、`KeyString`、Ristretto 调优字段、旧统计字段和 `ErrKeyStringRequired`
- [x] 1.3 使用 `ttlcache` 固定 TTL、最大 item 数和禁用 touch-on-hit 实现 `NewLoadingCache`、`GetOrLoad`、`Delete`、`Clear`、`Close`、`Name` 与 `Stats`，并保证成功写入同步可见
- [x] 1.4 实现按原始 comparable key 隔离的 `singleflight` entry 生命周期、内部 double-check、独立 `LoadTimeout`、caller context 取消和关闭后禁止写入语义
- [x] 1.5 重写 `common/runtime/localcache/cache_test.go`，覆盖配置错误、固定 TTL、容量上限、并发同 key 单次回源、不同 key 隔离、loader error 不缓存、caller 取消、delete/clear、自动 eviction 统计、关闭竞态及幂等关闭

## 2. 收敛共享指标

- [x] 2.1 更新 `common/runtime/observability/metrics/localcache.go`，只描述和采集 requests、loads、evictions、capacity，并让 success 直接读取 `Stats.LoadSuccess`
- [x] 2.2 重写 localcache collector 测试，验证四组 metric family、固定 label、`uint64` capacity、disabled provider 行为，并断言 singleflight 与 writes family 不再导出
- [x] 2.3 运行 `make common-test`，修复 common module 中全部新 API、并发和指标测试失败后再完成本组任务

## 3. 迁移 Auth Token Version Cache

- [x] 3.1 更新 `user-service/internal/features/auth/fx.go`，使用 `NewLoadingCache[string, int64]`、非泛型 `Config` 和 `uint64` 最大 item 数，删除 key encoder 与 clone 参数
- [x] 3.2 更新 `DirectTokenVersionCache.Stats()`、validator、测试 fixture 和 fake `StatsSource`，将逐次回源统计改为 `LoadSuccess`/`LoadError` 并保持关闭缓存失效语义
- [x] 3.3 迁移 auth 相关构造、Fx graph、provider、health 和 route helper 测试中的旧 `localcache.New`、`Config[K]` 与旧 Stats 字段
- [x] 3.4 运行 auth validators、auth application、auth infrastructure 和 auth Fx 相关 Go package 测试，确认 token version 主事实、Redis 回填、失效与 fail-closed 行为不变

## 4. 迁移 RBAC User-Role Cache

- [x] 4.1 更新 `user_role_cache.go` 与 resolver 类型，使用 `NewLoadingCache[uuid.UUID, []uuid.UUID]` 和 `uint64` 最大 item 数，删除 UUID 字符串化和 common clone callback
- [x] 4.2 将 `cloneRoleIDs` 明确应用于 loader 入缓存和 `RolesForUser` 出缓存边界，并保证 disabled/direct 路径同样返回独立 slice
- [x] 4.3 更新 RBAC direct stats、policy fixture、lifecycle 与 provider 测试中的旧 API 和旧 Stats 字段
- [x] 4.4 增加或重写 RBAC 测试，验证修改返回 slice 不污染后续读取、同 UUID 并发回源只执行一次、loader error 不缓存、失效与关闭继续 fail-closed
- [x] 4.5 运行 permission infrastructure/casbin 及相关 permission package 测试，确认 policy load、user-role invalidation 和多副本同步边界未改变

## 5. 同步部署观测资产与文档

- [x] 5.1 更新 user-service metrics 注册、health、route 和 Fx graph 测试期望，只保留 requests、loads、evictions、capacity 四组 localcache 指标
- [x] 5.2 更新 `deployments/observability/grafana` 的源 dashboard，删除 singleflight、writes 和 rejected/set-dropped panel 或查询，并重新生成 Compose provisioning dashboard
- [x] 5.3 更新 Prometheus alert、真实 metrics load 脚本及 `docs/observability/user-service-runbook.md`，删除旧 metric family、event label 和 Ristretto 排障语义
- [x] 5.4 运行 `make compose-dashboard-check` 及相关 metrics load/dashboard 校验，确认生成物无 drift 且仓库中不再引用删除的 metric family
- [x] 5.5 运行 `make user-service-architecture-lint`，确认 common、auth、permission、observability 与文档边界符合当前架构规则

## 6. 全仓验证与交付

- [x] 6.1 全仓扫描并删除残留的 `localcache.New`、`localcache.Cache`、`Config[...]`、`KeyString`、`CloneFunc`、`NumCounters`、`BufferItems`、旧 Stats 字段、Ristretto `Wait()` 及已删除指标引用
- [x] 6.2 运行 `make user-service-test`，修复 user-service 中全部编译、单元、集成和 Fx graph 回归
- [x] 6.3 检查 `git diff`，确认没有意外修改 HTTP API、OpenAPI 生成物、Ent schema、Atlas migration 或无关文件
- [x] 6.4 将本次预期代码、依赖锁文件、测试、观测资产、文档和 OpenSpec artifacts 暂存，然后运行 `make lint`；只有命令成功后才完成本任务
- [x] 6.5 保持本次预期变更已暂存并运行 `make verify`；只有完整验证成功且最终 drift 检查通过后才完成本任务
