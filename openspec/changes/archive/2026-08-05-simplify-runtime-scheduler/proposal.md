## Why

当前 `common/runtime/scheduler` 已覆盖本地防重叠、全局并发控制、Redis 分布式锁、重试退避、自动续租、任务 context 和优雅关闭，但单次执行依赖集中式 `jobRunState` 跟踪多种资源标志，生命周期与 cleanup 分支较多，新增或验证策略组合需要理解整条状态机。应在完整保留现有调度、重试和分布式锁功能的前提下，利用 `robfig/cron` 的 chain、panic recovery 和 drain 能力重组内部执行管线，降低实现复杂度并修正已发现的关闭与耗时语义问题。

## What Changes

- 完整保留本地 overlap、全局并发 `skip/wait`、分布式锁 `skip/wait`、Redis 重试/指数退避/jitter、锁自动续租、续租失败策略和全部 scheduler metrics/alerts。
- **BREAKING** 压缩公开配置中的冗余状态：以 nil/non-nil lock 与 renewal policy 表达启停，以正数 `WaitTimeout` 表达等待锁，移除可产生矛盾组合的 `Enabled`、`Mode` 和 `AutoRenew` 开关；不提供旧 API 兼容层。
- **BREAKING** 以固定任务 key 作为公开注册/删除标识，不再向调用方暴露底层 `cron.EntryID`；注册、启动、删除和停止方法使用简洁一致的命名。
- 使用 `robfig/cron` 继续负责现有可选 seconds parser、descriptors、时区、触发、动态增删、panic recovery 和底层 drain；AegisCore 内部使用不可导出的 invocation pipeline 串联 overlap、全局并发、锁、context、续租、观测、任务和 cleanup。
- 让每个 pipeline stage 通过词法 `defer` 只拥有并释放自身资源，删除集中式 `jobRunState`、资源持有布尔标志和跨阶段 cleanup 分支。
- 简化 Redis lock 重试循环与 Lua script 复用，同时保持等待上限、最大尝试次数、指数退避、jitter、owner token 校验、续租和释放行为。
- 修正 shutdown：首次停止新触发后取消任务并建立共享 drain，首次调用超时不影响后续调用继续等待。
- 修正执行耗时：completed/failed duration 从任务 started 时刻计算，不包含 overlap、全局并发或锁等待时间；现有 event、status、reason 和 `lock_renew_failed` 契约保持不变。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 保留 scheduler 的全部并发、锁、重试和续租功能，将稳定行为重述为分阶段执行管线并修正重复关闭语义。
- `runtime-observability`: 保留 scheduler 指标和告警事件集合，明确 duration 只度量实际任务执行时间并维持锁续租失败观测。

## Impact

- Go 代码：重组 `common/runtime/scheduler/` 的公开配置和内部执行结构，保留并简化 Redis lock/retry/renew 实现及测试；`common/runtime/observability/metrics/` 接口与事件集合不删除。
- 共享契约：这是不兼容 API 重构，但不是功能裁剪；当前仓库扫描未发现 scheduler 生产注册者，因此不提供旧字段、旧方法或类型别名。
- 依赖：继续使用当前 `github.com/robfig/cron/v3` 与 `github.com/redis/go-redis/v9`，不新增调度、锁或并发控制依赖。
- 观测与部署：保留 `aegiscore_scheduler_jobs_total`、`aegiscore_scheduler_job_duration_seconds`、scheduler job failed 和 lock renew failed 告警；只同步 duration 语义说明和测试，不删除 dashboard/alert 查询。
- 文档与规格：更新 `common/README.md`、`docs/ARCHITECTURE.md` 及 `shared-platform-primitives`、`runtime-observability` 规格，明确功能保持与内部所有权边界。
- 不影响 HTTP API、数据库 schema/migration、Ent/OpenAPI 生成物、认证/RBAC 安全边界、Docker/Kubernetes/Helm 部署清单。
