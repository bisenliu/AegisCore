# Tasks

## 1. Preparation

- [x] 1.1 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md`、本 change 的 `proposal.md` 和 `design.md`。
- [x] 1.2 确认本 change 使用 `docs/changes/export-runtime-dependency-metrics/`，不新增 `openspec/` 或 `docs/opsx/`。
- [x] 1.3 梳理 `common/runtime/observability/metrics.Provider`、`Provider.Register`、disabled 行为和 label 常量。
- [x] 1.4 梳理 `user-service/internal/providers` 中 `user_db`、`cache_redis`、metrics provider 和 route wiring。
- [x] 1.5 梳理 `common/runtime/workerpool.Stats()`、`common/runtime/scheduler.Metrics`、RBAC watcher `Running/LastError` 和 Casbin engine `Reload/LastError`。
- [x] 1.6 确认本变更不改变 health probe 语义、不新增业务指标、不新增 dashboard/alert/deployment artifact。

## 2. Label And Naming Contract

- [x] 2.1 在 `common/runtime/observability/metrics` 增加低基数 label key 常量：`resource`、`pool`、`job`、`event`、`status`、`reason`。
- [x] 2.2 更新 `ValidateLowCardinalityLabelKey` 测试，覆盖新增 label key。
- [x] 2.3 文档明确新增 label 的允许值必须来自固定资源名、pool 名、job key 或枚举状态。
- [x] 2.4 文档明确禁止用户 ID、角色 ID、权限 ID、Redis key、SQL、DSN、错误消息全文等高基数/敏感值。

## 3. PostgreSQL Metrics

- [x] 3.1 在 common metrics package 新增 SQL DB stats collector 或注册 helper。
- [x] 3.2 collector 输入包含固定 `Resource` 和 `*sql.DB`。
- [x] 3.3 导出 open connections、in-use、idle 和 max open gauges。
- [x] 3.4 导出 wait count、wait duration、max idle closed、max idle time closed 和 max lifetime closed counters。
- [x] 3.5 collector 不执行 SQL、不读取 DSN、不改变 pool 配置。
- [x] 3.6 增加单元测试覆盖 collector describe/collect 和 `resource=user_db` label。

## 4. Redis Metrics

- [x] 4.1 在 common metrics package 新增 Redis ping collector 或最小 ping adapter。
- [x] 4.2 collector 输入包含固定 `Resource`、ping timeout 和 Redis ping client。
- [x] 4.3 导出 `aegiscore_redis_up` gauge，成功为 1，失败为 0。
- [x] 4.4 导出 ping duration 指标，优先使用 histogram 或最近一次 gauge。
- [x] 4.5 导出 ping failure counter。
- [x] 4.6 scrape 时使用 timeout context，timeout 必须为正值。
- [x] 4.7 不在 label 中记录 Redis key、command 参数、addr、DB number、payload 或错误消息。
- [x] 4.8 增加成功、失败和 timeout 行为测试。

## 5. Workerpool Metrics

- [x] 5.1 在 common metrics package 新增 workerpool stats collector。
- [x] 5.2 collector 只依赖窄 `Stats() workerpool.Stats` source。
- [x] 5.3 导出 tasks total counter，event 覆盖 submitted、rejected、started、completed、failed、panicked。
- [x] 5.4 导出 queued、running、free、waiting、closed 和 workers gauges。
- [x] 5.5 pool label 使用固定 `auth_session_purge_pool` 或 `Stats().Name`，不得来自用户输入。
- [x] 5.6 增加 fake stats source 单元测试。
- [x] 5.7 确认不修改 workerpool 执行模型、Submit/Stop 语义或现有测试预期。

## 6. Scheduler Metrics Adapter

- [x] 6.1 在 common metrics package 实现 `scheduler.Metrics` Prometheus adapter。
- [x] 6.2 provider nil 或 disabled 时返回 `scheduler.NopMetrics{}`。
- [x] 6.3 `JobRegistered`、`JobTriggered`、`JobStarted`、`JobCompleted`、`JobFailed`、`JobSkipped`、`JobLockRenewFailed` 均记录 counter。
- [x] 6.4 `JobCompleted` 和 `JobFailed` 记录 duration histogram。
- [x] 6.5 `job` label 使用 scheduler job key，要求调用方只传固定枚举式任务名。
- [x] 6.6 `reason` label 只使用 scheduler 现有固定原因，其他事件使用固定 `none`。
- [x] 6.7 增加 adapter 单元测试，覆盖每个 Metrics 方法、disabled nop 和 label 值。
- [x] 6.8 确认不修改 scheduler 核心运行逻辑或现有测试预期。

## 7. RBAC Watcher Metrics

- [x] 7.1 为 RBAC watcher `Running/LastError` 增加 Prometheus collector 或服务侧 collector。
- [x] 7.2 导出 watcher running gauge。
- [x] 7.3 导出 watcher last error gauge，`LastError()!=nil` 为 1，否则 0。
- [x] 7.4 label 仅使用固定 resource，例如 `rbac_policy_watcher`，不记录错误消息。
- [x] 7.5 增加 watcher status collector 测试，覆盖 running/stopped 和 error/no error。
- [x] 7.6 确认不改变 `/readyz`、`/startupz` 对 watcher 的现有判断。

## 8. Casbin Policy Reload Metrics

- [x] 8.1 在 permission Casbin engine 中增加可选 reload metrics 窄接口或等价 hook。
- [x] 8.2 未注入 metrics 时使用 nop recorder。
- [x] 8.3 `Reload` 成功时记录 success counter 和 last status。
- [x] 8.4 `Reload` 失败时记录 failure counter 和 last status。
- [x] 8.5 metrics label 只使用 `status=success|failure`，不记录 role ID、permission ID、route template 或错误消息。
- [x] 8.6 更新 Casbin engine tests，覆盖 reload success/failure 对 metrics recorder 的调用。
- [x] 8.7 确认 reload failure preserve previous policy 的现有行为不变。

## 9. User-Service Wiring

- [x] 9.1 在 `user-service/internal/providers` 新增 runtime dependency metrics collector wiring。
- [x] 9.2 接收 `*commonmetrics.Provider`、`*sql.DB name:"user_db"`、`*redis.Client name:"cache_redis"` 和 `*config.Config`。
- [x] 9.3 注册 PostgreSQL collector，resource 使用 `resources.NameUserDB`。
- [x] 9.4 注册 Redis collector，resource 使用 `resources.NameCacheRedis`，timeout 复用 Redis ping timeout。
- [x] 9.5 暴露或注入 auth session purge pool stats source，并注册 workerpool collector，pool 使用 `auth_session_purge_pool`。
- [x] 9.6 注册 RBAC watcher status collector。
- [x] 9.7 在 permission feature Fx 中为 Casbin engine 注入 reload metrics recorder。
- [x] 9.8 如用户服务已有 scheduler provider，则注入 scheduler Prometheus adapter；如没有真实 scheduler job，仅保留 common adapter 和文档说明。
- [x] 9.9 所有 collector 通过 `Provider.Register` 注册；注册错误应让 Fx 启动失败。
- [x] 9.10 Metrics disabled 时不启动 Redis 后台 probe，不改变服务启动和 health check 语义。

## 10. Documentation

- [x] 10.1 更新 `common/runtime/observability/metrics/README.md`，说明 datastore、workerpool、scheduler、RBAC watcher 和 Casbin reload 指标。
- [x] 10.2 更新 `docs/ARCHITECTURE.md`，说明用户服务 `/metrics` 可暴露运行时依赖指标，health probe 语义不变。
- [x] 10.3 如 `docs/DEVELOPMENT.md` 有 metrics 配置说明，同步补充 runtime dependency metrics 和 label 基数约束。
- [x] 10.4 明确本变更不新增 dashboard、alert rules、ServiceMonitor、PodMonitor、Helm chart 或 Kubernetes artifact。
- [x] 10.5 确认文档没有重新引入 OpenSpec/OPSX 流程或目录。

## 11. Tests

- [x] 11.1 运行并更新 common metrics package 测试。
- [x] 11.2 增加 SQL collector、Redis collector、workerpool collector 和 scheduler adapter 单元测试。
- [x] 11.3 增加 user-service provider wiring 测试，确认 `/metrics` 包含固定资源/pool/RBAC 指标。
- [x] 11.4 增加 Casbin reload metrics recorder 测试。
- [x] 11.5 增加 RBAC watcher metrics 测试。
- [x] 11.6 回归 workerpool 和 scheduler 现有测试。
- [x] 11.7 确认 tests 不依赖真实 PostgreSQL/Redis，除非使用已有 testing containers 并显式标注。

## 12. Verification

- [x] 12.1 格式化修改过的 Go 文件：

```bash
gofmt -w <changed-go-files>
```

- [x] 12.2 运行 common metrics 相关测试：

```bash
cd common && go test ./runtime/observability/metrics ./runtime/workerpool ./runtime/scheduler
```

- [x] 12.3 运行用户服务相关测试：

```bash
make test-user-service
```

- [x] 12.4 运行 common 模块测试：

```bash
make test-common
```

- [x] 12.5 如 provider wiring 或架构文档变化触发边界检查，运行：

```bash
make architecture-lint
```

- [x] 12.6 扫描确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 12.7 手动或测试环境启动 metrics enabled 的用户服务，确认 `/metrics` 能看到 PostgreSQL、Redis、workerpool、scheduler/RBAC/Casbin 指标。

## 13. Guardrails

- [x] 13.1 不新增通用 eventbus、outbox、worker system 或可靠投递框架。
- [x] 13.2 不改变 health probe 语义。
- [x] 13.3 不新增 dashboard、alert rules 或部署监控资源。
- [x] 13.4 不新增 auth/user/role/permission 业务指标。
- [x] 13.5 不在 label 中放高基数或敏感值。
- [x] 13.6 不修改 HTTP API 响应、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- [x] 13.7 不新增 `openspec/` 或 `docs/opsx/`。
