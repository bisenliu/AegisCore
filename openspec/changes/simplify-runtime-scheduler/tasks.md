## 1. 简化公开模型与 Cron 生命周期

- [x] 1.1 重构 `common/runtime/scheduler` 公开类型：保留全局并发、`Locker`/`Lock`、Redis locker、retry 和 metrics 能力，以 nil lock/renew policy 与 `WaitTimeout` 压缩 `Enabled`、`Mode`、`AutoRenew` 冗余状态，并更新 validation/default 归一化
- [x] 1.2 将任务注册 API 收敛为固定 key 驱动的 `Add`/`Remove`，隐藏 `cron.EntryID`，继续支持标准五字段、可选 seconds、descriptors、scheduler timezone 和 `CRON_TZ`
- [x] 1.3 使用 `cron.WithChain(cron.Recover(...))` 和现有 logger adapter 统一底层 panic/stack 日志，保持 Start 幂等、动态注册删除和停止后拒绝注册/重启语义

## 2. 构建等价功能的执行 Pipeline

- [x] 2.1 创建不可导出的 invocation/handler/middleware pipeline，在注册时按 triggered、本地 overlap、全局 gate、lock acquire/retry、task context、renew、started/result、task 的固定顺序构造执行链
- [x] 2.2 将本地 overlap 与全局 `skip/wait` gate 迁入独立 stage，每个 stage 在成功 acquire 后立即注册局部 defer，并保持 `local_overlap`、`global_concurrency_limit`、root cancellation 和 token 释放行为
- [x] 2.3 将 lock acquire、task context/timeout、renew guard、task error/renew error 合并和 unlock 迁入独立 stage，保持 lock busy/error、`ContinueOnFailure`、独立 unlock timeout 与逆序 cleanup 行为
- [x] 2.4 将执行结果观测迁入独立 stage，使 completed/failed duration 从 started 时刻计算；panic 必须先记录 failed，再重新 panic 交给 cron recovery，且所有内层资源 defer 必须完成
- [x] 2.5 删除被 pipeline 取代的集中式 `jobRunState`、资源持有布尔标志和条件 cleanup 分支，不导出通用 middleware 或允许调用方重排安全关键 stage

## 3. 简化但保留 Redis Lock、重试与续租

- [x] 3.1 重构 Redis acquire wait loop，使用派生 deadline context 和可停止 timer 保持总 `WaitTimeout`、parent cancellation、`MaxAttempts`、指数退避上限与 jitter 行为
- [x] 3.2 将 unlock/renew Lua 提升为复用的 `redis.Script`，保持随机 owner token、Redis key builder、TTL、非 owner `ErrLockNotOwned` 和 Cluster-capable `redis.UniversalClient` 行为
- [x] 3.3 将续租循环封装为 invocation 局部 guard，保持 interval/operation timeout 默认值、`JobLockRenewFailed`、失败取消或继续、停止等待 goroutine和最终错误合并语义

## 4. 覆盖全部功能组合

- [x] 4.1 使用公开 API 覆盖空/重复 key、nil task、负 timeout、五/六字段、descriptors、timezone、`CRON_TZ`、按 key 删除、nil/non-nil lock/renew policy 和停止后拒绝注册，不为测试新增生产 hook
- [x] 4.2 使用 channel、原子状态和明确 deadline 覆盖默认 overlap skip、显式 overlap、全局 concurrency skip/wait、root cancellation，以及各 stage 在 error/panic 后释放 local/global token
- [x] 4.3 覆盖 lock skip/wait、总等待上限、最大尝试次数、backoff/jitter 边界、parent cancellation、Redis error、owner unlock/renew、lost ownership 和 Cluster-capable client
- [x] 4.4 覆盖续租成功、默认 interval/timeout、失败取消、`ContinueOnFailure`、task error 与 renew error 合并、panic cleanup、独立 unlock timeout 和 renew goroutine drain
- [x] 4.5 覆盖 duration 不含 gate/lock wait、Start 幂等、Stop 停止新触发、活动任务取消、首次 Stop timeout 后续 Stop 继续等待同一 drain，并运行 `cd common && go test -race ./runtime/scheduler`

## 5. 同步观测、架构与文档

- [x] 5.1 保持 `common/runtime/scheduler.Metrics` 与 `common/runtime/observability/metrics` 的全部 event/status/reason，包括 `lock_renew_failed`，更新测试以验证实际执行 duration 和低基数标签未因重构漂移
- [x] 5.2 更新 `common/README.md`、`docs/ARCHITECTURE.md` 和必要的 scheduler/观测说明，描述内部 pipeline、压缩配置、等待 goroutine风险和 Redis lease 边界，不得写成删除全局并发、重试、锁或续租功能
- [x] 5.3 审计 Prometheus alerts、Grafana 源/Compose dashboard、metrics load 脚本和 runbook 继续覆盖 scheduler job failed 与 lock renew failed，运行 `make compose-dashboard-check` 和 `cd common && go test ./runtime/observability/metrics`

## 6. 规格与最终质量门禁

- [x] 6.1 检查 proposal、design、两个 spec delta 与最终代码一致，运行 `openspec validate simplify-runtime-scheduler`、`openspec list --specs`、`openspec validate --specs` 和 `make user-service-architecture-lint`
- [x] 6.2 对修改的 Go 文件执行 `gofmt`，运行相关 package 测试与 `git diff --check`，确认仓库内不存在误删 retry、distributed lock、renew 或 `lock_renew_failed` 能力的残留变更
- [x] 6.3 检查 `git status --short`，仅暂存本次 change 的预期文件并复核 staged diff；不得暂存或覆盖用户无关变更
- [x] 6.4 在预期变更已暂存后运行 `make lint`；只有命令成功时才能勾选本任务
- [x] 6.5 在预期变更已暂存后运行 `make verify`，确认生成物 drift 与最终 `git diff --exit-code` 门禁通过；只有命令成功时才能勾选本任务并将 change 视为实现完成
